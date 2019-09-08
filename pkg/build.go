package pkg

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/flosch/pongo2"
	"github.com/pkg/errors"
)

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
		err = os.MkdirAll(destDir, 777)
		if err != nil {
			return errors.Wrapf(err, "Failed to create destination directory [%s]", destDir)
		}

		// If it's not a jinja file, we assume it's a static file and can be simply copied over
		if filepath.Ext(path) != ".jinja" {
			destPath := filepath.Join(config.OutputFolder, relPath)
			log.Printf("Copying %s -> %s\n", relPath, destPath)
			return copyFile(path, destPath)
		}

		// Otherwise, do the template rendering
		destPath := filepath.Join(config.OutputFolder, relPath[0:len(relPath)-len(filepath.Ext(relPath))])
		log.Printf("Rendering template %s -> %s\n", relPath, destPath)

		template, err := templateSet.FromFile(path)
		if err != nil {
			return errors.Wrapf(err, "Failed to parse template file [%s]", path)
		}

		destFile, err := os.Create(destPath)
		if err != nil {
			return errors.Wrapf(err, "Failed to create dest file for writing [%s]", destPath)
		}
		defer destFile.Close()

		err = template.ExecuteWriter(pongo2.Context{}, destFile)
		if err != nil {
			return errors.Wrapf(err, "Failed to render and write template file [%s]", path)
		}

		return nil
	})
	if err != nil {
		return errors.Wrapf(err, "Failed to walk content folder")
	}

	return nil
}
