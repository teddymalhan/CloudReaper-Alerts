# Packaging for CloudReaper binaries: host CLIs (cloudreaper, detector, sender) and
# provided.al2023 Lambda zips (scanner, notifier, reactor).
#
#   make package    # everything
#   make notifier   # just the notifier Lambda zip
#   make detector   # just the detector CLI

GO         ?= go
BUILD_DIR  := build
LAMBDA_DIR := terraform/build

# Lambda arch follows the host so the zips run on local Floci; override for
# real AWS if the function arch differs, e.g. `make lambdas LAMBDA_GOARCH=amd64`.
LAMBDA_GOARCH ?= $(shell $(GO) env GOARCH)

# -trimpath/-buildvcs=false keep rebuilds at the same source byte-identical;
# the fixed mtime + `zip -X` extend that to the archive, so Terraform's
# source_code_hash only changes when the code does.
GO_BUILD_FLAGS := -trimpath -buildvcs=false -ldflags "-s -w"
ZIP_EPOCH      := 200001010000

.PHONY: package cli lambdas cloudreaper detector sender scanner notifier reactor test clean

package: cli lambdas

cli: $(BUILD_DIR)/cloudreaper $(BUILD_DIR)/detector $(BUILD_DIR)/sender
lambdas: $(LAMBDA_DIR)/scanner.zip $(LAMBDA_DIR)/notifier.zip $(LAMBDA_DIR)/reactor.zip

cloudreaper: $(BUILD_DIR)/cloudreaper
detector:    $(BUILD_DIR)/detector
sender:      $(BUILD_DIR)/sender
scanner:     $(LAMBDA_DIR)/scanner.zip
notifier:    $(LAMBDA_DIR)/notifier.zip
reactor:     $(LAMBDA_DIR)/reactor.zip

$(BUILD_DIR)/%: FORCE
	$(GO) build $(GO_BUILD_FLAGS) -o $@ ./cmd/$*

$(LAMBDA_DIR)/%.zip: FORCE
	@mkdir -p $(LAMBDA_DIR)/$*
	GOOS=linux GOARCH=$(LAMBDA_GOARCH) CGO_ENABLED=0 \
	  $(GO) build $(GO_BUILD_FLAGS) -tags lambda.norpc -o $(LAMBDA_DIR)/$*/bootstrap ./cmd/$*
	@TZ=UTC touch -t $(ZIP_EPOCH) $(LAMBDA_DIR)/$*/bootstrap
	@rm -f $@
	cd $(LAMBDA_DIR)/$* && TZ=UTC zip -q -X ../$(notdir $@) bootstrap

test:
	$(GO) test ./...

clean:
	rm -rf $(BUILD_DIR) $(LAMBDA_DIR)

FORCE:
