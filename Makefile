.PHONY: all gendb swagger swagger-tool fmt fmt-check lint test test-backend test-frontend build check e2e

# 接口文档生成器版本，需与 .github/workflows/ci.yml 中的固定版本保持一致。
SWAG_VERSION ?= v1.16.6
SWAG ?= swag

all: check

gendb:
	@cd server && gentool -c gen.yml

# 安装固定版本的 swag CLI，供 make swagger 使用。
swagger-tool:
	@go install github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)

# 根据 handler 注解重新生成接口规范；生成物需提交入库。
swagger:
	@cd server && $(SWAG) init -g main.go -d cmd/api,internal -o internal/docs --outputTypes go,json --quiet

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

e2e:
	@cd web && npm run test:e2e
