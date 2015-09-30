ARGS=$(filter-out $@,$(MAKECMDGOALS))
BRANCH=`git rev-parse --abbrev-ref HEAD`
ENV=`basename "$PWD"`
NAME ?= lpg-load-balancer
PACKAGE_NAME ?= $(NAME)
PACKAGE_CONFLICT ?= $(PACKAGE_NAME)-beta
REVISION := $(shell git rev-parse --short HEAD || echo unknown)
LAST_TAG := $(shell git describe --tags --abbrev=0)
COMGPLv3S := $(shell echo `git log --oneline $(LAST_TAG)..HEAD | wc -l`)
VERSION := $(shell (cat VERSION || echo dev) | sed -e 's/^v//g')
ifneq ($(RELEASE),true)
    VERSION := $(shell echo $(VERSION)~beta.$(COMGPLv3S).g$(REVISION))
endif
ITTERATION := $(shell date +%s)
BUILD_PLATFORMS ?= -os="linux" -os="darwin" -os="windows" -os="freebsd"
DEB_PLATFORMS ?= debian/wheezy debian/jessie ubuntu/precise ubuntu/trusty ubuntu/utopic ubuntu/vivid
DEB_ARCHS ?= amd64 i386 arm armhf
RPM_PLATFORMS ?= el/6 el/7 ol/6 ol/7
RPM_ARCHS ?= x86_64 i686 arm armhf

all: deps test lint toolchain build

run:
	dogo

run-http-test:
	@wrk -c 1 -d 30s -t 1 --latency --timeout 1 http://127.0.0.1:8080

register-fake-http:
	curl -X PUT --user lb:7eNQ4iWLgDw4Q6w -d 'url=127.0.0.1:8081' -s http://127.0.0.1:8182 && echo
	curl -X PUT --user lb:7eNQ4iWLgDw4Q6w -d 'url=127.0.0.1:8082' -s http://127.0.0.1:8182 && echo
	curl -X GET --user lb:7eNQ4iWLgDw4Q6w -s http://127.0.0.1:8182 && echo

help:
	# make all => deps test lint toolchain build
	# make version - show information about current version
	# make deps - install all dependencies
	# make test - run project tests
	# make lint - check project code style
	# make toolchain - install crossplatform toolchain
	# make build - build project for all supported OSes
	# make package - package project using FPM

version: FORCE
	@echo Current version: $(VERSION)
	@echo Current iteration: $(ITTERATION)
	@echo Current revision: $(REVISION)

deps:
	# Installing dependencies...
	go get github.com/tools/godep
	go get -u github.com/golang/lint/golint
	go get github.com/mitchellh/gox
	# go get golang.org/x/tools/cmd/cover
	go get golang.org/x/sys/windows/svc
	godep restore

toolchain:
	# Building toolchain...
	gox $(BUILD_PLATFORMS)

build:
	gox $(BUILD_PLATFORMS) \
		-ldflags "-X main.NAME $(PACKAGE_NAME) -X main.VERSION $(VERSION) -X main.REVISION $(REVISION)" \
		-output="out/binaries/$(NAME)-{{.OS}}-{{.Arch}}"

lint:
	# Checking project code style...
	golint ./... | grep -v "be unexported"

test:
	# Running tests...
	go test ./... -cover

build-and-deploy:
	make build BUILD_PLATFORMS="-os=linux -arch=amd64"
	make package-deb-fpm ARCH=amd64 PACKAGE_ARCH=amd64
	make package-rpm-fpm ARCH=amd64 PACKAGE_ARCH=amd64

package: package-deps package-deb package-rpm

package-deb:
	# Building Debian compatible packages...
	make package-deb-fpm ARCH=amd64 PACKAGE_ARCH=amd64
	make package-deb-fpm ARCH=386 PACKAGE_ARCH=i386
	make package-deb-fpm ARCH=arm PACKAGE_ARCH=arm
	make package-deb-fpm ARCH=arm PACKAGE_ARCH=armhf

package-rpm:
	# Building RedHat compatible packages...
	make package-rpm-fpm ARCH=amd64 PACKAGE_ARCH=amd64
	make package-rpm-fpm ARCH=386 PACKAGE_ARCH=i686
	make package-rpm-fpm ARCH=arm PACKAGE_ARCH=arm
	make package-rpm-fpm ARCH=arm PACKAGE_ARCH=armhf

package-deps:
	# Installing packaging dependencies...
	gem install fpm

package-deb-fpm:
	@mkdir -p out/deb/
	fpm -s dir -t deb -n $(PACKAGE_NAME) -v $(VERSION) \
		-p out/deb/$(PACKAGE_NAME)_$(PACKAGE_ARCH).deb \
		--deb-priority optional --category admin \
		--force \
		--deb-compression bzip2 \
		--url https://gitlab.com/gotlium/lpg-load-balancer \
		--description "lpg-load-balancer" \
		-m "GoTLiuM InSPiRiT <gotlium@gmail.com>" \
		--license "GPLv3" \
		--vendor "github.com/gotlium" \
		--conflicts $(PACKAGE_CONFLICT) \
		--provides lpg-load-balancer \
		--replaces lpg-load-balancer \
		--depends ca-certificates \
		--depends git \
		--deb-suggests docker \
		-a $(PACKAGE_ARCH) \
		packaging/root/=/ \
		out/binaries/$(NAME)-linux-$(ARCH)=/usr/bin/lpg-load-balancer

package-rpm-fpm:
	@mkdir -p out/rpm/
	fpm -s dir -t rpm -n $(PACKAGE_NAME) -v $(VERSION) \
		-p out/rpm/$(PACKAGE_NAME)_$(PACKAGE_ARCH).rpm \
		--rpm-compression bzip2 --rpm-os linux \
		--force \
		--url https://gitlab.com/gotlium/lpg-load-balancer \
		--description "lpg-load-balancer" \
		-m "GoTLiuM InSPiRiT <gotlium@gmail.com>" \
		--license "GPLv3" \
		--vendor "github.com/gotlium" \
		--conflicts $(PACKAGE_CONFLICT) \
		--provides lpg-load-balancer \
		--replaces lpg-load-balancer \
		-a $(PACKAGE_ARCH) \
		packaging/root/=/ \
		out/binaries/$(NAME)-linux-$(ARCH)=/usr/bin/lpg-load-balancer

install-ci-tools:
	bash < <(curl -s -S -L https://raw.githubusercontent.com/moovweb/gvm/master/binscripts/gvm-installer)
	gvm install go1.5.1
	gvm use go1.5.1
	gvm pkgset create lpg-load-balancer
	gvm pkgset use lpg-load-balancer

run_ci_test:
	@test -f gvm.inst || (make install-ci-tools && make deps && touch gvm.inst)
	@source ~/.gvm/scripts/gvm; gvm use go1.5.1; go test ./... -cover

session:
	@tmux list-sessions | grep LPG-LB-WIN >& /dev/null && tmux attach -t "LPG-LB-WIN" || true
	@tmux new-session -s "LPG-LB-WIN" -n "LPG-LB-FR" -d
	@tmux split-window -h -t "LPG-LB-WIN"
	@tmux split-window -v -t "LPG-LB-WIN"
	@tmux select-pane -t "LPG-LB-WIN":"LPG-LB-FR".0
	@tmux split-window -v -t "LPG-LB-WIN"
	@tmux send-keys -t "LPG-LB-WIN":"LPG-LB-FR".0 "go run main.go http -l :8081" C-m
	@tmux send-keys -t "LPG-LB-WIN":"LPG-LB-FR".1 "go run main.go http -l :8082" C-m
	@tmux send-keys -t "LPG-LB-WIN":"LPG-LB-FR".2 "go run main.go run" C-m
	@tmux send-keys -t "LPG-LB-WIN":"LPG-LB-FR".3 "sleep 5; make register-fake-http" C-m
	@tmux send-keys -t "LPG-LB-WIN":"LPG-LB-FR".3 "make run-http-test"
	@tmux attach -t "LPG-LB-WIN"

pull:
	@git pull origin `git rev-parse --abbrev-ref HEAD`
	@git log --name-only -1|grep migrations >& /dev/null && ./manage.py migrate --noinput || true
	@test -f touch.reload && touch touch.reload || true

push:
	@git status --porcelain|grep -v '??' && (echo '\033[0;32mCommit message:\033[0m' && MSG=`rlwrap -o -S "> " cat` && git commit -am "$$MSG") || true
	@git push origin $(BRANCH) || (git pull origin $(BRANCH) && git push origin $(BRANCH))

reset:
	@git rev-parse --abbrev-ref HEAD|grep master >& /dev/null && false || true
	@git reset --soft `git log --pretty=format:"%H" master.. | tail -1`
	@git status --porcelain|grep -v '??' && (echo '\033[0;32mCommit message:\033[0m' && MSG=`rlwrap -o -S "> " cat` && git commit -am "$$MSG") || true
	@git push origin `git rev-parse --abbrev-ref HEAD` -f

pull-from-reset:
	@git rev-parse --abbrev-ref HEAD|grep master >& /dev/null && false || true
	@git stash
	@git reset --hard `git log --pretty=format:"%H" master.. | tail -1`
	@git pull origin `git rev-parse --abbrev-ref HEAD` -f
	@test -f touch.reload && touch touch.reload || true


FORCE:
