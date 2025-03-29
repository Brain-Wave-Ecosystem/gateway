# Pull proto commit from proto repository commands
proto-pull:
	git submodule update --remote --force proto

buf-gen:
	git submodule update --remote --force proto && cd ./proto && make buf-gen

# Docker-Compose commands
gateway-up:
	docker-compose -f ./deployments/compose/gateway.docker-compose.yaml --env-file=./.env up -d --build

gateway-down:
	docker-compose -f ./deployments/compose/gateway.docker-compose.yaml --env-file=./.env down