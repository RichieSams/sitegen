package pkg

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

type codeFormattingConfig struct {
	ChromaStyle string `yaml:"chroma_style"`
	TabWidth    int    `yaml:"tab_width"`
}

type configDataEntry struct {
	Pattern       string `yaml:"pattern"`
	SortKey       string `yaml:"sort_key"`
	SortAscending bool   `yaml:"sort_ascending"`
}

type buildConfig struct {
	ContentFolder   string                     `yaml:"content_folder"`
	TemplatesFolder string                     `yaml:"templates_folder"`
	OutputFolder    string                     `yaml:"output_folder"`
	CodeFormatting  codeFormattingConfig       `yaml:"code_formatting"`
	Data            map[string]configDataEntry `yaml:"data"`
}

func parseConfig(filePath string) (buildConfig, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return buildConfig{}, errors.Wrapf(err, "Failed to open the config file [%s]", filePath)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	var config buildConfig
	err = decoder.Decode(&config)
	if err != nil {
		return buildConfig{}, errors.Wrapf(err, "Failed to decode config file [%s]", filePath)
	}

	if config.ContentFolder == "" {
		return buildConfig{}, errors.Errorf("content_folder is a required parameter in the config file")
	}
	if config.OutputFolder == "" {
		return buildConfig{}, errors.Errorf("output_folder is a required parameter in the config file")
	}

	configAbsPath, err := filepath.Abs(filePath)
	if err != nil {
		return buildConfig{}, errors.Wrapf(err, "Failed to get the abs path of the config file [%s]", filePath)
	}
	configDir := filepath.Dir(configAbsPath)

	if !filepath.IsAbs(config.ContentFolder) {
		config.ContentFolder = filepath.Join(configDir, config.ContentFolder)
	}
	if !filepath.IsAbs(config.OutputFolder) {
		config.OutputFolder = filepath.Join(configDir, config.OutputFolder)
	}
	if config.TemplatesFolder == "" {
		config.TemplatesFolder = configDir
	} else if !filepath.IsAbs(config.TemplatesFolder) {
		config.TemplatesFolder = filepath.Join(configDir, config.TemplatesFolder)
	}

	if config.CodeFormatting.ChromaStyle == "" {
		config.CodeFormatting.ChromaStyle = "monokai"
	}

	if config.CodeFormatting.TabWidth == 0 {
		config.CodeFormatting.TabWidth = 4
	}

	return config, nil
}
