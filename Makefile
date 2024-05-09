DOCKER?=docker

generate:
	rm -rf pkg/api/openapi
	$(DOCKER) run --rm -v ${PWD}:/local:rw openapitools/openapi-generator-cli:v6.0.1 generate -i /local/config/schema/voyager_file_schema_v1.0.0.yaml -g go -o /local/pkg/api/openapi --skip-validate-spec --global-property=models
	gofmt -w pkg/api/openapi
.PHONY: generate


binary:
	go build -o bin/voyager cmd/main.go
.PHONY: binary