module github.com/viperadnan-git/gogpm/cmd/gpcli

go 1.25

require (
	github.com/creativeprojects/go-selfupdate v1.5.1
	github.com/knadh/koanf/parsers/yaml v1.1.0
	github.com/knadh/koanf/providers/file v1.2.0
	github.com/knadh/koanf/providers/structs v1.0.0
	github.com/knadh/koanf/v2 v2.3.0
	github.com/urfave/cli/v3 v3.6.1
	github.com/viperadnan-git/gogpm v0.0.0
)

require (
	code.gitea.io/sdk/gitea v0.22.0 // indirect
	github.com/42wim/httpsig v1.2.3 // indirect
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/davidmz/go-pageant v1.0.2 // indirect
	github.com/fatih/structs v1.1.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-fed/httpsig v1.1.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/google/go-github/v30 v30.1.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.8 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/ulikunitz/xz v0.5.14 // indirect
	github.com/xanzy/go-gitlab v0.115.0 // indirect
	go.yaml.in/yaml/v3 v3.0.3 // indirect
	golang.org/x/crypto v0.41.0 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/time v0.12.0 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// Use local library during development
replace github.com/viperadnan-git/gogpm => ../..
