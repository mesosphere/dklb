# detect what platform we're running in so we can use proper command flavors
OS := $(shell uname -s)
ifeq ($(OS),Linux)
PLATFORM := linux
SHA1 := sha1sum
endif
ifeq ($(OS),Darwin)
PLATFORM := darwin
SHA1 := shasum -a1
endif
