# goctl Tools

## Prerequisites

```bash
which goctl && goctl --version
go install github.com/zeromicro/go-zero/tools/goctl@latest
```

For `damai-go`, all `goctl` generation must use `--style go_zero`. Do not follow legacy mixed naming in existing directories.

## create_api_service

Create REST API service with goctl

```bash
mkdir -p <output_dir> && cd <output_dir>
goctl api new <service_name> --style go_zero
go mod tidy
go build ./...
```

## create_rpc_service

Create gRPC service with goctl

```bash
mkdir -p <output_dir> && cd <output_dir>
goctl rpc new <service_name> --style go_zero
go mod tidy
go build ./...
```

Or from existing proto:

```bash
goctl rpc protoc <file>.proto --go_out=. --go-grpc_out=. --zrpc_out=. --style go_zero
go mod tidy
go build ./...
```

## generate_api_from_spec

Generate code from `.api` file

```bash
goctl api go -api <file>.api -dir . --style go_zero
go mod tidy
go build ./...
```

Safe to re-run; it will not overwrite custom handler or logic files.

## generate_model

Generate database model from table

**MySQL (live DB):**

```bash
goctl model mysql datasource \
  -url "user:pass@tcp(host:3306)/db" \
  -table "<table>" -dir ./model --style go_zero
```

**MySQL (DDL file):**

```bash
goctl model mysql ddl -src <file>.sql -dir ./model --style go_zero
```

**With cache:** add `-cache` flag.

**PostgreSQL:**

```bash
goctl model pg datasource \
  -url "postgres://user:pass@host:5432/db?sslmode=disable" \
  -table "<table>" -dir ./model --style go_zero
```

**MongoDB:**

```bash
goctl model mongo -type <TypeName> -dir ./model --style go_zero
```

## validate_api_spec

```bash
goctl api validate -api <file>.api
```

## analyze_project

```bash
goctl api doc -dir .
find . -name "*.api" -o -name "*.proto" -o -name "*.go" | head -50
```

## Post-Generation Checklist

After every generation:

1. `[ ! -f go.mod ] && go mod init <module>`
2. `go mod tidy`
3. Verify imports match `go.mod` module path
4. `go build ./...`
5. Check generated file names remain in `go_zero` snake_case
