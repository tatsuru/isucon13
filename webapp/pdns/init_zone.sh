#!/usr/bin/env bash

set -eux
cd $(dirname $0)

if test -f /home/isucon/env.sh; then
	. /home/isucon/env.sh
fi

ISUCON_SUBDOMAIN_ADDRESS=${ISUCON13_POWERDNS_SUBDOMAIN_ADDRESS:-127.0.0.1}

sed 's/<ISUCON_SUBDOMAIN_ADDRESS>/'$ISUCON_SUBDOMAIN_ADDRESS'/g' u.isucon.dev.zone.initial > u.isucon.dev.zone
sudo pdns_control bind-add-zone u.isucon.dev /home/isucon/isucon13/webapp/pdns/u.isucon.dev.zone
sudo pdns_control bind-reload-now u.isucon.dev
