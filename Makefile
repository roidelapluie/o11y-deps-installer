.PHONY: packer download_packer build_packer_image build_ansible_alpine_tar build

PACKER_VERSION := 1.8.6
PACKER_URL := https://releases.hashicorp.com/packer/$(PACKER_VERSION)/packer_$(PACKER_VERSION)_linux_amd64.zip

build:
	@echo "Building Go project..."
	CGO_ENABLED=0 go build -ldflags '-extldflags "-static"'
	@echo "Go project built successfully."

packer: download_packer build_packer_image build_ansible_alpine_tar

download_packer:
	@echo "Downloading Packer..."
	curl -sSLo packer_$(PACKER_VERSION)_linux_amd64.zip $(PACKER_URL)
	unzip -o packer_$(PACKER_VERSION)_linux_amd64.zip packer
	rm packer_$(PACKER_VERSION)_linux_amd64.zip
	chmod +x packer
	@echo "Packer downloaded successfully."

build_packer_image:
	@echo "Building Packer image..."
	./packer build packer_ansible_alpine.json
	gzip ansible_alpine.tar
	@echo "ansible_alpine.tar.gz built successfully."

