# Development environment

## 1. Install dependencies and Go runtime

### For Debian/Ubuntu
```bash
apt-get install -y curl bison gcc
bash < <(curl -s -S -L https://raw.githubusercontent.com/moovweb/gvm/master/binscripts/gvm-installer)
source /root/.gvm/scripts/gvm
gvm install go1.4.3
gvm use go1.4.3
gvm pkgset create l0xyd
gvm pkgset use l0xyd
```

### For OSX if you have brew
```
brew install go
```

### For FreeBSD
```
pkg install go-1.5.1 gmake git mercurial
```

## 2. Download L0xyd sources

```
go get git.lpgenerator.ru/sys/l0xyd
cd ~/.gvm/pkgsets/go1.4.3/l0xyd/src/git.lpgenerator.ru/sys/l0xyd
```

## 4. Install L0xyd dependencies

This will download and restore all dependencies required to build l0xyd:

```
make deps
```

**For FreeBSD use `gmake deps`**

## 5. Run L0xyd

Normally you would use `l0xyd`, in order to compile and run Go source use go toolchain:

```
go run main.go run
```

You can run l0xyd in debug-mode:

```
go run --debug main.go run
```

## 6. Compile and install L0xyd binary

```
go build
go install
```

## 7. Congratulations!
