# Pull proto commit from proto repository
proto-pull:
	git submodule update --remote --force proto

buf-gen:
	git submodule update --remote --force proto && cd ./proto && make buf-gen