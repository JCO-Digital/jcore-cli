build: install
	pnpm run build

watch: install
	pnpm run watch

install:
	pnpm i

clean:
	rm bin/*
