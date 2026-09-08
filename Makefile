.PHONY: all gendb fmt fmt-check lint test test-backend test-frontend build check

all: check

gendb:
	@cd server && gentool -c gen.yml

# 格式化会修改工作区；提交前运行一次。
fmt:
	@cd server && gofmt -w $$(find . -name '*.go' -type f)
	@cd web && npm run format

fmt-check:
	@unformatted="$$(cd server && gofmt -l $$(find . -name '*.go' -type f))"; test -z "$$unformatted" || { echo "请运行 make fmt："; echo "$$unformatted"; exit 1; }
	@cd web && npm run format:check

lint:
	@cd server && go vet ./...
	@cd web && npm run lint

test: test-backend test-frontend

test-backend:
	@cd server && go test -race -count=1 ./...

test-frontend:
	@cd web && npm run test

build:
	@cd server && mkdir -p tmp && go build -o ./tmp/api ./cmd/api
	@cd web && npm run build

check: fmt-check lint test build
