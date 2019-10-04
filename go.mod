module github.com/RichieSams/sitegen

go 1.13

require (
	github.com/Depado/bfchroma v1.1.2
	github.com/alecthomas/chroma v0.6.0
	github.com/flosch/pongo2 v0.0.0-20190707114632-bbf5a6c351f4
	github.com/gorilla/mux v1.7.3
	github.com/juju/errors v0.0.0-20190806202954-0232dcc7464d // indirect
	github.com/pkg/errors v0.8.1
	github.com/radovskyb/watcher v1.0.7
	github.com/satori/go.uuid v1.2.0
	github.com/spf13/cobra v0.0.5
	gopkg.in/russross/blackfriday.v2 v2.0.1
	gopkg.in/yaml.v2 v2.2.2
)

replace gopkg.in/russross/blackfriday.v2 => github.com/russross/blackfriday/v2 v2.0.1
