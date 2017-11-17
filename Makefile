GOBUILDFLAGS=
GC=go build
SRC=main.go
PROG=bitarb

$(PROG): $(SRC)
	go get ./...
	$(GC) $(GOBUILDFLAGS) -o $(PROG) $(SRC)
	chmod +x $(PROG)

clean: 
	rm bitarb 2>&1 >/dev/null
