.PHONY: release
release:
	@read -p "Enter release version (e.g., v1.0.0): " VERSION; \
	git add .; \
	git commit -m "Release $$VERSION"; \
	git tag $$VERSION; \
	git push origin $$VERSION
	git push

.PHONY: test
test:
	go test -v ./...
