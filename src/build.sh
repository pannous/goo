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

cd /opt/other/go/src/ 
# /opt/other/go/bin/go build -x -a -o ../pkg/darwin_arm64/fmt.a fmt
# /opt/other/go/bin/go build -x -a -o ../pkg/darwin_arm64/errors.a errors
# /opt/other/go/bin/go build -x -a -o ../pkg/darwin_arm64/slices.a slices
# /opt/other/go/bin/go build -x -a -o ../pkg/darwin_arm64/strconv.a strconv

echo rebuild all
# /opt/other/go/bin/go install -a -x std
/opt/other/go/bin/go install -a std

# /opt/other/go/bin/go build -o ../pkg/darwin_arm64/fmt.a fmt
# /opt/other/go/bin/go build -o ../pkg/darwin_arm64/errors.a errors
# /opt/other/go/bin/go build -o ../pkg/darwin_arm64/slices.a slices
# /opt/other/go/bin/go build -o ../pkg/darwin_arm64/strconv.a strconv


/opt/other/go/bin/go tool compile \
  -I $GOROOT/pkg/darwin_arm64 \
  -p main \
  -complete \
  -o /tmp/test.o /opt/other/go/goo/tests.goo

/opt/other/go/bin/go tool link \
  -L $GOROOT/pkg/darwin_arm64 \
  -o /tmp/test_exec /tmp/test.o

# cd $GOROOT/src/cmd/compile
# /opt/other/go/bin/go build -gcflags="all=-N -l" 
# ./compile -p main -complete -o /tmp/test.o /opt/other/go/goo/tests.goo
# /opt/other/go/bin/go tool link -L $GOROOT/pkg/darwin_arm64/ -o /tmp/test_exec /tmp/test.o
# # /opt/other/go/bin/go tool link -L $GOROOT/pkg/obj/go-bootstrap/darwin_arm64/ -o /tmp/test_exec /tmp/test.o
# /tmp/test_exec