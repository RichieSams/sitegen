package pkg

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/radovskyb/watcher"
)

// EchoHandler takes the body and writes it back to the client
type EchoHandler struct{}

func (h *EchoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Failed to read body: %s\n", err)
		return
	}

	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.Write(body)
}

func createConfigFileWatcher(configPath string, inputFoldersWatcher **watcher.Watcher) (*watcher.Watcher, error) {
	w := watcher.New()
	w.SetMaxEvents(1)
	err := w.Add(configPath)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to add file watcher for [%s]", configPath)
	}

	go func() {
		err = w.Start(500 * time.Millisecond)
		if err != nil {
			log.Fatal(errors.Wrapf(err, "Failed to start file watcher for folders"))
		}
	}()

	go func() {
		w.Wait()

		for {
			select {
			case <-w.Event:
				log.Println("Config file changed. Re-installing input folder watcher, and re-building")
				(*inputFoldersWatcher).Close()

				config, err := parseConfig(configPath)
				if err != nil {
					log.Fatal(err)
				}

				*inputFoldersWatcher, err = createInputFoldersWatcher(config, configPath)
				if err != nil {
					log.Fatal(err)
				}

				err = BuildSite(configPath)
				if err != nil {
					log.Fatal(err)
				}
			case err := <-w.Error:
				log.Fatal(err)
			case <-w.Closed:
				return
			}
		}
	}()

	return w, nil
}

func createInputFoldersWatcher(config buildConfig, configPath string) (*watcher.Watcher, error) {
	log.Println("create input")
	w := watcher.New()
	w.SetMaxEvents(1)
	err := w.AddRecursive(config.ContentFolder)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to add file watcher for [%s]", config.ContentFolder)
	}
	err = w.AddRecursive(config.TemplatesFolder)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to add file watcher for [%s]", config.TemplatesFolder)
	}
	log.Println("Start input")

	go func() {
		err = w.Start(500 * time.Millisecond)
		if err != nil {
			log.Fatal(errors.Wrapf(err, "Failed to start file watcher for folders"))
		}
	}()

	go func() {
		w.Wait()

		for {
			select {
			case <-w.Event:
				log.Println("Triggering re-build")
				err = BuildSite(configPath)
				if err != nil {
					log.Fatalln(err)
				}
			case err := <-w.Error:
				log.Fatalln(err)
			case <-w.Closed:
				return
			}
		}
	}()

	return w, nil
}

// Serve serves the files fileServeDir via GET requests
// and echos any POST / PUT requests
func Serve(configPath string, servePort int) error {
	config, err := parseConfig(configPath)
	if err != nil {
		return err
	}

	// Install a file watcher on the initial output folder
	inputFoldersWatcher, err := createInputFoldersWatcher(config, configPath)
	if err != nil {
		return err
	}
	defer inputFoldersWatcher.Close()

	configFileWatcher, err := createConfigFileWatcher(configPath, &inputFoldersWatcher)
	if err != nil {
		return err
	}
	defer configFileWatcher.Close()

	// Start up a simple web server
	r := mux.NewRouter()

	r.PathPrefix("/").Handler(http.FileServer(http.Dir(config.OutputFolder))).Methods("GET", "HEAD")
	r.PathPrefix("/").Handler(&EchoHandler{}).Methods("PUT", "POST")

	log.Printf("Serving %s on HTTP port: %d\n", config.OutputFolder, servePort)
	srv := &http.Server{
		Handler: r,
		Addr:    fmt.Sprintf("127.0.0.1:%d", servePort),
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	return srv.ListenAndServe()
}
