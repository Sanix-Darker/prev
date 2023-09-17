# Install

You can install the pre-compiled binary (in several ways), use Docker or compile from source (when on OSS).

Below you can find the steps for each of them.

## Install the pre-compiled binary

=== "homebrew tap"

    ```bash
    brew install sanix-darker/tap/prev
    ```

=== "apt"

    ```bash
    echo 'deb [trusted=yes] https://apt.fury.io/sanix-darker/ /' | sudo tee /etc/apt/sources.list.d/sanix-darker.list
    sudo apt update
    sudo apt install prev
    ```

=== "yum"

    ```bash
    echo '[sanix-darker]
    name=Gemfury sanix-darker repository
    baseurl=https://yum.fury.io/sanix-darker/
    enabled=1
    gpgcheck=0' | sudo tee /etc/yum.repos.d/sanix-darker.repo
    sudo yum install goreleaser
    ```

## deb, rpm and apk packages

Download the .deb, .rpm or .apk packages from the [release page](https://github.com/sanix-darker/prev/releases) and install them with the appropriate tools.

## Manually

=== "go install"

    ```bash
    go install github.com/sanix-darker/prev@latest
    ```

=== "Released tar file"

    Download the pre-compiled binaries from the [release page](https://github.com/sanix-darker/prev/releases) page and copy them to the desired location.
    ```bash
    $ VERSION=v1.0.0
    $ OS=Linux
    $ ARCH=x86_64
    $ TAR_FILE=prev_${OS}_${ARCH}.tar.gz
    $ wget https://github.com/sanix-darker/prev/releases/download/${VERSION}/${TAR_FILE}
    $ sudo tar xvf ${TAR_FILE} prev -C /usr/local/bin
    $ rm -f ${TAR_FILE}
    ```

=== "manually"

    ```bash
    $ git clone github.com/sanix-darker/prev
    $ cd prev
    $ go generate ./...
    $ go install
    ```
