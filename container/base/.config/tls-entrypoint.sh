#!/bin/bash

# This should be blank to allow webserver to start
PASSPHRASE=""

# Renew certificates that expire within this many seconds (30 days).
RENEW_BEFORE=2592000

# The CA is trusted by hand in each browser/OS, so replacing it means
# importing it again - say so loudly.
if [ -f "$CERT_DIR/jcoCA.pem" ] && ! openssl x509 -checkend "$RENEW_BEFORE" -noout -in "$CERT_DIR/jcoCA.pem" >/dev/null; then
  echo "********************************************************************"
  echo "The JCore CA certificate expires within 30 days; creating a new one."
  echo "Import the new ~/.config/jcore/ssl/jcoCA.pem into your browser/OS"
  echo "trust store again, replacing the old one."
  echo "********************************************************************"
  rm -f "$CERT_DIR/jcoCA.key" "$CERT_DIR/jcoCA.pem" "$CERT_DIR/jcoCA.srl"
fi

# Create CA key and Cert if it doesn't exist
if [ ! -f "$CERT_DIR/jcoCA.key" ] || [ ! -f "$CERT_DIR/jcoCA.pem" ]; then
  # Create password file if it doesn't exist
  if [ ! -f "$CERT_DIR/password.txt" ]; then
    cat /dev/urandom | tr -dc 'a-zA-Z0-9!@#$%^&*' | fold -w 32 | head -n 1 >"$CERT_DIR/password.txt"
    echo "\n" >>"$CERT_DIR/password.txt"
  fi

  echo "[req]
    default_bits = 2048
    prompt = no
    default_md = sha256
    distinguished_name = dn

    [dn]
    C = FI
    ST = Uusimaa
    L = Raseborg
    O = JCO Digital
    OU = Hosting
    emailAddress = developer@jco.fi
    CN = JCO
    " >"$CERT_DIR/jcoCA.conf"

  # Generate Key
  openssl genpkey -algorithm rsa -des3 -pkeyopt rsa_keygen_bits:2048 -pass "file:$CERT_DIR/password.txt" -out "$CERT_DIR/jcoCA.key"

  # Generate Cert. Browsers don't limit the lifetime of a root you trust
  # yourself, so make it long-lived: replacing it means re-importing it.
  openssl req -new -config "$CERT_DIR/jcoCA.conf" -x509 -sha256 -nodes -days 3650 -key "$CERT_DIR/jcoCA.key" -out "$CERT_DIR/jcoCA.pem" -passin "file:$CERT_DIR/password.txt"
fi

# (Re)create the host key and cert when missing, close to expiry, not
# signed by the current CA (e.g. it was just renewed), or issued for a
# different set of domains than the project now has.
HOST_DOMAINS="$LOCAL_DOMAIN $DOMAINS"
RENEW_HOST=""
if [ ! -f "$SSL_DIR/host.key" ] || [ ! -f "$SSL_DIR/host.crt" ]; then
  RENEW_HOST="missing"
elif ! openssl x509 -checkend "$RENEW_BEFORE" -noout -in "$SSL_DIR/host.crt" >/dev/null; then
  RENEW_HOST="expires within 30 days"
elif ! openssl verify -CAfile "$CERT_DIR/jcoCA.pem" "$SSL_DIR/host.crt" >/dev/null 2>&1; then
  RENEW_HOST="not signed by the current CA"
elif [ "$(cat "$SSL_DIR/host.domains" 2>/dev/null)" != "$HOST_DOMAINS" ]; then
  RENEW_HOST="domains changed"
fi

if [ -n "$RENEW_HOST" ]; then
  echo "Creating host key and certificate ($RENEW_HOST)."

  DNS=1
  ALTNAME="DNS.${DNS} = localhost"
  for hostName in ${DOMAINS}; do
    DNS=$((DNS + 1))
    ALTNAME="$ALTNAME
  DNS.${DNS} = ${hostName}"
    DNS=$((DNS + 1))
    ALTNAME="$ALTNAME
  DNS.${DNS} = *.${hostName}"
  done

  echo "[req]
  default_bits = 2048
  prompt = no
  default_md = sha256
  x509_extensions = v3_req
  distinguished_name = dn

  [dn]
  C = FI
  ST = Uusimaa
  L = Raseborg
  O = JCO Digital
  OU = Hosting
  emailAddress = developer@jco.fi
  CN = $LOCAL_DOMAIN

  [v3_req]
  subjectAltName = @alt_names

  [alt_names]
  $ALTNAME
  " >"$SSL_DIR/host.conf"
  cat "$SSL_DIR/host.conf"

  echo "authorityKeyIdentifier=keyid,issuer
  basicConstraints=CA:FALSE
  keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
  subjectAltName = @alt_names

  [alt_names]
  $ALTNAME
  " >"$SSL_DIR/v3.ext"
  cat "$SSL_DIR/v3.ext"

  # Generate the Private Key
  openssl genpkey -algorithm rsa -pkeyopt rsa_keygen_bits:2048 -out "$SSL_DIR/host.key"

  # Generate the CSR
  openssl req -new -config "$SSL_DIR/host.conf" -key "$SSL_DIR/host.key" -out "$SSL_DIR/host.csr" -passin pass:$PASSPHRASE

  # Generate the Certificate
  openssl x509 -req -extfile "$SSL_DIR/v3.ext" -days 365 -in "$SSL_DIR/host.csr" -CA "$CERT_DIR/jcoCA.pem" -CAkey "$CERT_DIR/jcoCA.key" -CAcreateserial -out "$SSL_DIR/host.crt" -sha256 -passin "file:$CERT_DIR/password.txt"

  echo "$HOST_DOMAINS" >"$SSL_DIR/host.domains"
fi

chown -R jcore:jcore "$CERT_DIR" "$SSL_DIR"
