# Development environment

## 1. Install dependencies and Go runtime

### For Debian/Ubuntu
```bash
apt-get install -y mercurial git-core wget make
wget https://storage.googleapis.com/golang/go1.5.1.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go*-*.tar.gz
```

### For OSX if you have brew
```
brew install go
```

### For FreeBSD
```
pkg install go-1.5.1 gmake git mercurial
```

## 2. Configure Go

Add to `.profile` or `.bash_profile`:

```bash
export GOPATH=$HOME/Go
export PATH=$PATH:$GOPATH/bin:/usr/local/go/bin
```

Create new terminal session and create $GOPATH directory:

```
mkdir -p $GOPATH
```

## 3. Download lpg-load-balancer sources

```
go get git.lpgenerator.ru/sys/lpg-load-balancer
cd $GOPATH/src/git.lpgenerator.ru/sys/lpg-load-balancer/
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
