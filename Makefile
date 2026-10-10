BINARY_NAME=contribution-tracker
CACHE_FILE=.github_cache.json
OUTPUT_DIR=output
CONFIG?=config.json
TEX_TEMPLATE?=templates/project_centric.tex.tmpl

.PHONY: all build run fresh clean

all: run

build:
	go build -o $(BINARY_NAME) main.go

run: build
	./$(BINARY_NAME) -config $(CONFIG) -template $(TEX_TEMPLATE) -outdir $(OUTPUT_DIR)

compile:
	xelatex $(OUTPUT_DIR)/$(TEX_FILE)

fresh: build
	rm -f $(CACHE_FILE)
	./$(BINARY_NAME) -config $(CONFIG) -template $(TEX_TEMPLATE) -outdir $(OUTPUT_DIR)

clean:
	rm -f $(BINARY_NAME) *.tex *.pdf $(CACHE_FILE)
	rm -rf $(OUTPUT_DIR)
	rm -f *.aux *.log *.out *.toc