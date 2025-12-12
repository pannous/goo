# echo "full rebuild changes version string"
cd src
./make.bash

../bin/go build -o /dev/null strings fmt slices strconv # just for caching through away the output

# NOW activate transforms!
./build-compiler.sh 