# codeit

Minimal Go project scaffold.

## Structure

- `cmd/codeit`: application entrypoint
- `internal`: private application code
- `pkg`: public libraries for reuse
- `api`: API definitions such as OpenAPI or protobuf files
- `configs`: configuration templates and examples
- `scripts`: development and automation scripts
- `docs`: project documentation
- `test`: integration and end-to-end test assets

## Run

```powershell
go run ./cmd/codeit
```

## Next step

If this project will be published or imported elsewhere, update the module path in `go.mod` to your repository path.