GO_BUILD_TAGS=remote containers_image_openpgp exclude_graphdriver_btrfs

build:
	mkdir -p bin
	go build -tags "$(GO_BUILD_TAGS)" -o bin/app

run: build
	./bin/app

test:
	go test -tags "$(GO_BUILD_TAGS)" -v ./... -count=1
