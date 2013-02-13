---
layout: default
title: goxc
posts: 5
---

goxc
====

goxc cross-compiles Go programs to (up to) 9 target platforms at once.

goxc is written in Go but using *os.exec()* to call 'go build'.

goxc was inspired by Dave Cheney's Bash script [golang-crosscompile](https://github.com/davecheney/golang-crosscompile).
BUT, goxc crosscompiles to all platforms at once. The artifacts are saved into a folder structure along with a markdown file of relative links.

Pre-requisites
--------------
At this stage you still need to:

 1. Use Linux (I haven't tested on other platforms)
 2. Download the go source code and set up the GOROOT accordingly.

Basic Usage
-----------

### Run once:

To build the toolchains for all 9 platforms:

       goxc -t

### Now build your artifacts

To build binaries for your app:

       cd path/to/app/folder
       goxc .

### Going further

See 'goxc -h' for more options.

Note that if running from source, replace 'goxc' with 'go run goxc.go'

Outcome
-------

Artifacts generated into:

 (outputfolder)/(version)/(OS)/(ARCH)/appname(.exe)

By default, the output folder is ($GOBIN)/appname, and the version is 'latest', but you can specify these.

e.g.

      goxc -av=0.1 -d=/home/me/myapp/ghpages/downloads/ .

Limitations
-----------

 * Currently there's a bug meaning you can only run goxc on the current folder.
 * Only tested on Linux. No idea if it would work on Win/Mac/FreeBSD
 * Currently goxc is only designed to build standalone Go apps without linked libraries. You can try but YMMV

Todo
----

 * Fix bug to allow building of a different directory (other than current working directory)
 * 'specify artifact folder' option
 * 'specify version name' option
 * 'generate Downloads page' option
 * 'download golang source' option?
 * building .so/.dll shared libraries?

See also
--------

See [my golang-crosscompile fork](https://github.com/laher/golang-crosscompile) for an added 'build-all' task similar to goxc.