package pkg

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/alecthomas/chroma"
	chroma_html "github.com/alecthomas/chroma/formatters/html"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/flosch/pongo2"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	markdown_html "github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"gopkg.in/yaml.v2"
)

var frontMatterRe = regexp.MustCompile(`(?msU)\+\+\+[\r\n]+(.*)+\+\+\+`)
var templateDoubleParenRe = regexp.MustCompile(`(?msU).*({{.*}}).*`)
var templateParenPercentRe = regexp.MustCompile(`(?msU).*({%.*%}).*`)

type frontMatterType map[string]interface{}

func parseFrontMatter(input []byte) (frontMatter frontMatterType, body []byte, err error) {
	matches := frontMatterRe.FindSubmatchIndex(input)
	if matches == nil || len(matches) < 4 {
		return map[string]interface{}{}, input, nil
	}

	frontMatterBytes := input[matches[2]:matches[3]]
	body = input[matches[1]:]

	if len(frontMatterBytes) == 0 {
		return map[string]interface{}{}, body, nil
	}

	err = yaml.Unmarshal(frontMatterBytes, &frontMatter)
	if err != nil {
		return map[string]interface{}{}, body, errors.Wrap(err, "Failed to parse frontmatter as YAML")
	}

	return frontMatter, body, nil
}

func copyFile(srcPath string, destPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return errors.Wrapf(err, "Failed to open source file for copying [%s]", srcPath)
	}
	defer srcFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return errors.Wrapf(err, "Failed to create dest file for copying [%s]", destPath)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return errors.Wrapf(err, "Failed to copy [%s] to [%s]", srcPath, destPath)
	}

	return nil
}

func renderJinjaFile(inputPath string, outputPath string, templateSet *pongo2.TemplateSet, templateData pongo2.Context) error {
	template, err := templateSet.FromFile(inputPath)
	if err != nil {
		return errors.Wrapf(err, "Failed to parse template file [%s]", inputPath)
	}

	destFile, err := os.Create(outputPath)
	if err != nil {
		return errors.Wrapf(err, "Failed to create dest file for writing [%s]", outputPath)
	}
	defer destFile.Close()

	err = template.ExecuteWriter(templateData, destFile)
	if err != nil {
		return errors.Wrapf(err, "Failed to render and write template file [%s]", inputPath)
	}

	return nil
}

// maskOutTemplateLanguage regex searches through the document and replaces any instances of `{{ ... }}` and `{% ... %}` with GUIDs
// We do this so the markdown renderer doesn't get confused and try to escape template language
func maskOutTemplateLanguage(inputDocument []byte) (maskedDocument []byte, maskedValues map[string][]byte) {
	maskedValues = map[string][]byte{}

	maskedDocument, maskedValues = maskDocument(inputDocument, templateDoubleParenRe, maskedValues)
	maskedDocument, maskedValues = maskDocument(maskedDocument, templateParenPercentRe, maskedValues)

	return maskedDocument, maskedValues
}

func maskDocument(inputDocument []byte, re *regexp.Regexp, maskedValues map[string][]byte) ([]byte, map[string][]byte) {
	maskedDocument := []byte{}

	matches := [][]int{}
	for _, match := range re.FindAllSubmatchIndex(inputDocument, -1) {
		matches = append(matches, []int{match[2], match[3]})
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i][0] < matches[j][0] })

	lastMaskedIndex := 0
	for _, matchIndices := range matches {
		maskedDocument = append(maskedDocument, inputDocument[lastMaskedIndex:matchIndices[0]]...)
		mask := uuid.NewV4().String()
		maskedValues[mask] = inputDocument[matchIndices[0]:matchIndices[1]]
		maskedDocument = append(maskedDocument, []byte(mask)...)

		lastMaskedIndex = matchIndices[1]
	}
	maskedDocument = append(maskedDocument, inputDocument[lastMaskedIndex:]...)

	return maskedDocument, maskedValues
}

func restoreTemplateLanguage(maskedDocument []byte, maskedValues map[string][]byte) (restoredDocument []byte) {
	restoredDocument = maskedDocument

	for uuid, value := range maskedValues {
		restoredDocument = bytes.ReplaceAll(restoredDocument, []byte(uuid), value)
	}

	return restoredDocument
}

type CodeHighlighterRenderer struct {
	Style     *chroma.Style
	Formatter *chroma_html.Formatter
	Errors    error
}

func NewCodeHighlighterRenderer(tabWidth int, style string) CodeHighlighterRenderer {
	return CodeHighlighterRenderer{
		Style:     styles.Get(style),
		Formatter: chroma_html.New(chroma_html.TabWidth(tabWidth)),
	}
}

func (r *CodeHighlighterRenderer) RenderNode(w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, bool) {
	// Skip all nodes that are not CodeBlock nodes
	codeBlock, ok := node.(*ast.CodeBlock)
	if !ok {
		return ast.GoToNext, false
	}

	lexer := lexers.Get(strings.Trim(string(codeBlock.Info), "\t "))
	if lexer == nil {
		lexer = lexers.Fallback
	}

	// Tokenize the code
	iterator, err := lexer.Tokenise(nil, string(codeBlock.Literal))
	if err != nil {
		r.Errors = multierror.Append(r.Errors, fmt.Errorf("Failed to tokenise code block - %w", err)).ErrorOrNil()
		return ast.GoToNext, false
	}

	if err := r.Formatter.Format(w, styles.Monokai, iterator); err != nil {
		r.Errors = multierror.Append(r.Errors, fmt.Errorf("Failed to format code block - %w", err)).ErrorOrNil()
		return ast.GoToNext, false
	}

	return ast.GoToNext, true
}

func renderMarkdownFile(inputPath string, outputPath string, templateSet *pongo2.TemplateSet, codeFormatting codeFormattingConfig, templateData pongo2.Context) error {
	markdownBytes, err := ioutil.ReadFile(inputPath)
	if err != nil {
		return errors.Wrapf(err, "Failed to read input markdown file [%s]", inputPath)
	}

	frontMatter, body, err := parseFrontMatter(markdownBytes)
	if err != nil {
		return err
	}

	templateExtendsVal, ok := frontMatter["template"]
	if !ok {
		return fmt.Errorf("Failed to render markdown file [%s]. `template` is a required parameter in the frontmatter", inputPath)
	}
	templateExtends, ok := templateExtendsVal.(string)
	if !ok {
		return fmt.Errorf("Failed to render markdown file [%s]. `template` should be a string path for the template to extend", inputPath)
	}
	delete(frontMatter, "template")

	// Force unix newlines
	// The markdown parser can't handle \r\n
	sanitizedBody := []byte(strings.ReplaceAll(string(body), "\r\n", "\n"))

	// Mask out template language prior to rendering markdown
	maskedDocument, maskedValues := maskOutTemplateLanguage(sanitizedBody)

	// Render the markdown
	extensions := parser.Tables | parser.FencedCode | parser.Strikethrough | parser.SpaceHeadings | parser.BackslashLineBreak | parser.DefinitionLists | parser.Footnotes | parser.NoIntraEmphasis | parser.MathJax
	parser := parser.NewWithExtensions(extensions)
	codeRenderer := NewCodeHighlighterRenderer(codeFormatting.TabWidth, codeFormatting.ChromaStyle)
	renderer := markdown_html.NewRenderer(markdown_html.RendererOptions{
		Flags:          markdown_html.CommonFlags,
		RenderNodeHook: codeRenderer.RenderNode,
	})
	content := markdown.ToHTML(maskedDocument, parser, renderer)

	// Check for code formatting errors
	if codeRenderer.Errors != nil {
		return fmt.Errorf("Failed to format one or more code blocks - %w", err)
	}

	// Restore the template language
	restoredDocument := restoreTemplateLanguage(content, maskedValues)

	// Render the final template
	templateString := fmt.Sprintf(`{%% extends "%s" %%}`, templateExtends)
	for key, value := range frontMatter {
		templateString += fmt.Sprintf(`
			{%% block %s %%}
			%v
			{%% endblock %%}`, key, value)
	}
	templateString += fmt.Sprintf(`
		{%% block content %%}
		%s
		{%% endblock %%}`, restoredDocument)

	template, err := templateSet.FromString(templateString)
	if err != nil {
		log.Printf("Template:\n\n%s\n\n", templateString)
		return errors.Wrapf(err, "Failed to parse template data for [%s]", inputPath)
	}

	destFile, err := os.Create(outputPath)
	if err != nil {
		return errors.Wrapf(err, "Failed to create dest file for writing [%s]", outputPath)
	}
	defer destFile.Close()

	err = template.ExecuteWriter(templateData, destFile)
	if err != nil {
		return errors.Wrapf(err, "Failed to render and write template file [%s]", inputPath)
	}

	return nil
}

func parseData(config buildConfig) (pongo2.Context, error) {
	context := pongo2.Context{}

	for entryName, entryInfo := range config.Data {
		files, err := filepath.Glob(filepath.Join(config.ContentFolder, entryInfo.Pattern))
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to Glob for data [%s], using pattern [%s]", entryName, entryInfo.Pattern)
		}

		dataEntry := []frontMatterType{}
		for _, file := range files {
			fileBytes, err := ioutil.ReadFile(file)
			if err != nil {
				return nil, errors.Wrapf(err, "Failed to read file [%s] for data", file)
			}

			frontMatter, _, err := parseFrontMatter(fileBytes)
			if err != nil {
				return nil, err
			}

			// Add extra info to the frontmatter
			outputPath, err := filepath.Rel(config.ContentFolder, file)
			if err != nil {
				return nil, errors.Wrapf(err, "Failed to get relative path of file [%s] for data", file)
			}
			if filepath.Ext(outputPath) == ".jinja" || filepath.Ext(outputPath) == ".md" {
				outputPath = outputPath[0 : len(outputPath)-len(filepath.Ext(outputPath))]
			}
			frontMatter["output_path"] = "/" + outputPath

			dataEntry = append(dataEntry, frontMatter)
		}

		// Sort the entries by their sort key
		if entryInfo.SortKey != "" {
			// Validate that the sort key entries are all strings
			for _, entry := range dataEntry {
				_, ok := entry[entryInfo.SortKey].(string)
				if !ok {
					return nil, fmt.Errorf("Can't sort entry %v by non-string value - %v", entry, entry[entryInfo.SortKey])
				}
			}

			sort.Slice(dataEntry, func(i, j int) bool {
				dateI := dataEntry[i][entryInfo.SortKey].(string)
				dateJ := dataEntry[j][entryInfo.SortKey].(string)

				if entryInfo.SortAscending {
					return dateI < dateJ
				}

				return dateJ < dateI
			})
		}

		context[entryName] = dataEntry
	}

	return context, nil
}

// BuildSite will parse the supplied config file and use it to generate a site
func BuildSite(configPath string) error {
	config, err := parseConfig(configPath)
	if err != nil {
		return err
	}

	// Nuke any existing output directory
	err = os.RemoveAll(config.OutputFolder)
	if err != nil {
		return errors.Wrapf(err, "Failed to delete existing output folder [%s]", config.OutputFolder)
	}

	// Create the jinja parsing setup
	templateLoader, err := pongo2.NewLocalFileSystemLoader(config.TemplatesFolder)
	if err != nil {
		return errors.Wrapf(err, "Failed to create template loader with basePath [%s]", config.TemplatesFolder)
	}
	templateSet := pongo2.NewSet("sitegen", templateLoader)

	// Parse any data
	templateData, err := parseData(config)
	if err != nil {
		return err
	}

	err = filepath.Walk(config.ContentFolder, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(config.ContentFolder, path)
		if err != nil {
			return err
		}

		destDir := filepath.Dir(filepath.Join(config.OutputFolder, relPath))
		err = os.MkdirAll(destDir, 0777)
		if err != nil {
			return errors.Wrapf(err, "Failed to create destination directory [%s]", destDir)
		}

		// If it's a jinja file, render the template as is
		if filepath.Ext(path) == ".jinja" {
			destPath := filepath.Join(config.OutputFolder, relPath[0:len(relPath)-len(filepath.Ext(relPath))])
			log.Printf("Rendering template %s -> %s\n", relPath, destPath)
			return renderJinjaFile(path, destPath, templateSet, templateData)
		}

		// If it's a md file, render the markdown and then use that to render a template
		if filepath.Ext(path) == ".md" {
			destPath := filepath.Join(config.OutputFolder, relPath[0:len(relPath)-len(filepath.Ext(relPath))])
			log.Printf("Rendering markdown template %s -> %s\n", relPath, destPath)
			return renderMarkdownFile(path, destPath, templateSet, config.CodeFormatting, templateData)
		}

		// If it's not a jinja file, we assume it's a static file and can be simply copied over
		destPath := filepath.Join(config.OutputFolder, relPath)
		log.Printf("Copying %s -> %s\n", relPath, destPath)
		return copyFile(path, destPath)
	})
	if err != nil {
		return errors.Wrapf(err, "Failed to walk content folder")
	}

	return nil
}
