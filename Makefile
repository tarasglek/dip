MYPKG=github.com/tarasglek/devingressproxy
CROSSCOMPILE=linux

dip.linux:
	$(MAKE) compile CROSSCOMPILE=linux OUTPUT=dip.linux

compile: vendor
	docker run --rm -it -v $(PWD):/go/src/$(MYPKG) -w /go/src/$(MYPKG) \
		--entrypoint sh instrumentisto/glide \
		-c "CGO_ENABLED=0 GOOS=$(CROSSCOMPILE) go build -o $(OUTPUT) -ldflags -s -a -installsuffix cgo ."

dip.mac:
	$(MAKE) compile CROSSCOMPILE=darwin OUTPUT=dip.mac

vendor:
	docker run --rm -it -v $(PWD):/go/src/$(MYPKG) -w /go/src/$(MYPKG) \
		instrumentisto/glide install

clean:
	rm dip.*