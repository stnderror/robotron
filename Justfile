run:
    watchexec -r -e go go run .

lint:
    golangci-lint run
