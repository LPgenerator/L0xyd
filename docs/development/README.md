# Development environment

## 1. Install dependencies and Go runtime

### For Debian/Ubuntu
```bash
apt-get install -y curl bison gcc
bash < <(curl -s -S -L https://raw.githubusercontent.com/moovweb/gvm/master/binscripts/gvm-installer)
source /root/.gvm/scripts/gvm
gvm install go1.4.3
gvm use go1.4.3
gvm pkgset create lpg-load-balancer
gvm pkgset use lpg-load-balancer
```

### For OSX if you have brew
```
brew install go
```

### For FreeBSD
```
pkg install go-1.5.1 gmake git mercurial
```

## 2. Download lpg-load-balancer sources

```
go get git.lpgenerator.ru/sys/lpg-load-balancer
cd ~/.gvm/pkgsets/go1.4.3/lpg-load-balancer/src/git.lpgenerator.ru/sys/lpg-load-balancer
```

## 4. Install lpg-load-balancer dependencies

This will download and restore all dependencies required to build lpg-load-balancer:

```
make deps
```

**For FreeBSD use `gmake deps`**

## 5. Run lpg-load-balancer

Normally you would use `lpg-load-balancer`, in order to compile and run Go source use go toolchain:

```
go run main.go run
```

You can run lpg-load-balancer in debug-mode:

```
go run --debug main.go run
```

## 6. Compile and install lpg-load-balancer binary

```
go build
go install
```

## 7. Congratulations!
