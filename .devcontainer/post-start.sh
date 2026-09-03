#!/bin/bash

set -o errexit
set -o pipefail
set -o nounset
# set -o xtrace

## Use nftables backend for iptables to ensure Docker networking works on Ubuntu 26.04+
## <https://github.com/orgs/sommerfeld-io/projects/1/views/1?pane=issue&itemId=184218287>
sudo update-alternatives --set iptables /usr/sbin/iptables-nft
sudo update-alternatives --set ip6tables /usr/sbin/ip6tables-nft

task cleanup:update
task symlinks
task compose:pull
