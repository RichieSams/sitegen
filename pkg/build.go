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

	"github.com/Depado/bfchroma"
	"github.com/alecthomas/chroma/formatters/html"
	"github.com/flosch/pongo2"
	"github.com/pkg/errors"
	"github.com/satori/go.uuid"
	bf "gopkg.in/russross/blackfriday.v2"
	"gopkg.in/yaml.v2"
)

var frontMatterRe = regexp.MustCompile(`(?msU)\+\+\+[\r\n]+(.*)+\+\+\+`)
var templateDoubleParenRe = regexp.MustCompile(`(?msU).*({{.*}}).*`)
var templateParenPercentRe = regexp.MustCompile(`(?msU).*({%.*%}).*`)

func parseFrontMatter(input []byte) (frontMatter map[string]interface{}, body []byte, err error) {
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

func renderJinjaFile(inputPath string, outputPath string, templateSet *pongo2.TemplateSet) error {
	template, err := templateSet.FromFile(inputPath)
	if err != nil {
		return errors.Wrapf(err, "Failed to parse template file [%s]", inputPath)
	}

	destFile, err := os.Create(outputPath)
	if err != nil {
		return errors.Wrapf(err, "Failed to create dest file for writing [%s]", outputPath)
	}
	defer destFile.Close()

	err = template.ExecuteWriter(pongo2.Context{}, destFile)
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

func renderMarkdownFile(inputPath string, outputPath string, templateSet *pongo2.TemplateSet, codeFormatting codeFormattingConfig) error {
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

	// Mask out template language prior to rendering markdown
	maskedDocument, maskedValues := maskOutTemplateLanguage(body)

	// Render the markdown
	extensions := bf.Tables | bf.FencedCode | bf.Strikethrough | bf.SpaceHeadings | bf.BackslashLineBreak | bf.DefinitionLists | bf.Footnotes | bf.NoIntraEmphasis

	content := bf.Run(
		maskedDocument,
		bf.WithRenderer(
			bfchroma.NewRenderer(
				bfchroma.WithoutAutodetect(),
				bfchroma.Style(codeFormatting.ChromaStyle),
				bfchroma.ChromaOptions(
					html.TabWidth(codeFormatting.TabWidth),
				),
				bfchroma.Extend(
					bf.NewHTMLRenderer(bf.HTMLRendererParameters{}),
				),
			),
		),
		bf.WithExtensions(extensions),
	)

	// Restore the template language
	restoredDocument := restoreTemplateLanguage(content, maskedValues)

	// Render the final template
	templateData := fmt.Sprintf(`{%% extends "%s" %%}`, templateExtends)
	for key, value := range frontMatter {
		templateData += fmt.Sprintf(`
			{%% block %s %%}
			%v
			{%% endblock %%}`, key, value)
	}
	templateData += fmt.Sprintf(`
		{%% block content %%}
		%s
		{%% endblock %%}`, restoredDocument)

	template, err := templateSet.FromString(templateData)
	if err != nil {
		log.Printf("Template:\n\n%s\n\n", templateData)
		return errors.Wrapf(err, "Failed to parse template data for [%s]", inputPath)
	}

	destFile, err := os.Create(outputPath)
	if err != nil {
		return errors.Wrapf(err, "Failed to create dest file for writing [%s]", outputPath)
	}
	defer destFile.Close()

	err = template.ExecuteWriter(pongo2.Context{}, destFile)
	if err != nil {
		return errors.Wrapf(err, "Failed to render and write template file [%s]", inputPath)
	}

	return nil
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
			return renderJinjaFile(path, destPath, templateSet)
		}

		// If it's a md file, render the markdown and then use that to render a template
		if filepath.Ext(path) == ".md" {
			destPath := filepath.Join(config.OutputFolder, relPath[0:len(relPath)-len(filepath.Ext(relPath))])
			log.Printf("Rendering markdown template %s -> %s\n", relPath, destPath)
			return renderMarkdownFile(path, destPath, templateSet, config.CodeFormatting)
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
