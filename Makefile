.PHONY: docker test docker-stop

test:
	go test -count=1 -p 1 -v ./...

docker:
	cd wstest && \
	docker run -it  --rm \
		-v "${PWD}/wstest/config:/config" \
		-v "${PWD}/wstest/reports:/reports" \
		-p 127.0.0.1:9001:9001 -p 127.0.0.1:8080:8080 \
		--name fuzzingserver \
		crossbario/autobahn-testsuite

docker-stop:
	docker stop $(shell docker ps -a -q --filter="name=fuzzingserver")
