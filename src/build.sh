# cd /opt/other/go/src
# GOEXPERIMENT=aliastypeparams,swissmap,synchashtriemap ./make.bash

export GOOS=darwin 
export GOARCH=arm64 
# export GOEXPERIMENT=aliastypeparams,swissmap,synchashtriemap
export GOROOT_FINAL=/opt/other/go
export GOTMPDIR=/tmp/go-debug
export GOROOT=/opt/other/go     
export GODEBUG=keepwork=1 
export WORK=/tmp/go-debug/
cd $GOROOT/src/cmd/compile
/opt/other/go/bin/go build -gcflags="all=-N -l" 
./compile -p main -complete -o /tmp/test.o /opt/other/go/goo/compiler_ok.goo
# /opt/other/go/bin/go tool link -L $GOROOT/pkg/obj/go-bootstrap/darwin_arm64/ -o /tmp/test_exec /tmp/test.o
# /tmp/test_exec