# AI Instructions for go-zero

## File Priority

1. `workflows.md` - Task patterns
2. `tools.md` - goctl commands
3. `patterns.md` - Code patterns
4. `zero-skills` - Detailed patterns

## Rules

### Spec-First

- ALWAYS create `.api` spec before code
- Write spec following patterns in `patterns.md`
- Validate with `goctl api validate`

### Tool Usage

- Use `goctl` commands in terminal, NOT manual code generation
- `goctl api new` for new API services
- `goctl rpc new` / `goctl rpc protoc` for new RPC services
- `goctl api go` for code from spec
- `goctl model mysql/pg/mongo` for database models
- Always run post-generation steps: `go mod tidy` -> verify imports -> `go build ./...`
- If `goctl` is not installed, install it:

```bash
go install github.com/zeromicro/go-zero/tools/goctl@latest
```

### Implementation

- Generate FULL implementation, not stubs
- Fill logic layer with business code
- Add validation tags: `validate:"required,email"`
- Generate tests automatically

### Documentation

- ALWAYS generate `README.md` for new services
- Include service overview and purpose
- Include API/RPC endpoint documentation
- Include configuration guide
- Include usage examples with `curl` / `grpcurl`
- Include testing instructions
- Generate `API.md` / `RPC.md` for detailed endpoint docs
- Include request/response examples
- Document error codes and handling

### Go-Zero Conventions

- Context first: `func(ctx context.Context, req *types.Request)`
- Errors: `errorx.NewCodeError(code, msg)`
- Config: `json:",default=value"`
- Validation: `validate:"required,min=3"`

## Decision Tree

```text
User Request
├─ New API?  -> Write .api spec -> goctl api go -> go mod tidy -> go build -> Generate docs
├─ New RPC?  -> Write .proto    -> goctl rpc protoc -> go mod tidy -> go build -> Generate docs
├─ Database? -> goctl model mysql/pg/mongo
└─ Modify?   -> Edit .api -> goctl api go -> go mod tidy -> go build -> Update docs
```

## Detailed Patterns

For complete implementation patterns, refer to `zero-skills`:

- REST API -> `references/rest-api-patterns.md`
- RPC Services -> `references/rpc-patterns.md`
- Database -> `references/database-patterns.md`
- Resilience -> `references/resilience-patterns.md`
- goctl Commands -> `references/goctl-commands.md`
- Troubleshooting -> `troubleshooting/common-issues.md`

## Avoid

- Empty stubs
- Missing validation
- `fmt.Errorf` for API errors
- Manual SQL instead of generated models when generation applies
- Missing context propagation
- Skipping post-generation steps
- Mismatched `--style` flag with existing code
