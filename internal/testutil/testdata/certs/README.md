## Test Certificates

1. Create a Certificate Authority (CA) Certificate and Key:

    ```bash
    # Generate the CA private key
    openssl genrsa -out ca-key.pem 4096
    
    # Create the CA self-signed certificate
    openssl req -x509 -new -nodes -key ca-key.pem -sha256 -days 3650 -out ca-cert.pem \
      -subj "/CN=MyCA"
    ```

2. Generate the Server Certificate and Key:

    ```bash
    # Generate the server private key
    openssl genrsa -out server-key.pem 4096
    
    # Create a certificate signing request (CSR) for the server
    openssl req -new -key server-key.pem -out server.csr \
      -subj "/CN=localhost"
    
    # Sign the server CSR with the CA certificate to create the server certificate
    openssl x509 -req -in server.csr -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial \
      -out server-cert.pem -days 365 -sha256 \
      -extfile <(printf "subjectAltName=DNS:localhost,IP:127.0.0.1,IP:::1,IP:0.0.0.0\nextendedKeyUsage=serverAuth")
    ```

3. Generate the Client Certificate:

    ```bash
    # Generate the client private key
    openssl genrsa -out client-key.pem 4096
    
    # Create a certificate signing request (CSR) for the client
    openssl req -new -key client-key.pem -out client.csr \
      -subj "/CN=client"
    
    # Sign the client CSR with the CA certificate to create the client certificate
    openssl x509 -req -in client.csr -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial \
      -out client-cert.pem -days 365 -sha256 \
      -extfile <(printf "extendedKeyUsage=clientAuth")
    ```
