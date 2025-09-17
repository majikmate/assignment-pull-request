# assignment-1

This is the first assignment in our series. Please review the overview diagram
below before starting.

Have a look at following video:

[![Watch on YouTube](https://img.youtube.com/vi/oT0qLaVUHO0/0.jpg)](https://www.youtube.com/watch?v=oT0qLaVUHO0)

![Assignment Overview](static/overview.png)

## Getting Started

Please get familiar with [mermaid](https://mermaid.js.org/intro/) documentation
to understand the diagram syntax used in this assignment.

Here's a sequence diagram that shows the HTTPS connection negotiation process:

```mermaid
sequenceDiagram
    participant Client
    participant Server
    
    Client->>Server: TCP Connection (Port 443)
    Server->>Client: TCP ACK
    
    Client->>Server: ClientHello (TLS version, cipher suites, random)
    Server->>Client: ServerHello (chosen cipher, random, certificate)
    Server->>Client: Certificate
    Server->>Client: ServerHelloDone
    
    Client->>Server: ClientKeyExchange (pre-master secret)
    Client->>Server: ChangeCipherSpec
    Client->>Server: Finished (encrypted)
    
    Server->>Client: ChangeCipherSpec
    Server->>Client: Finished (encrypted)
    
    Note over Client,Server: Secure HTTPS communication established
    
    Client->>Server: HTTP Request (encrypted)
    Server->>Client: HTTP Response (encrypted)
```

Here's a class diagram showing the Go standard library HTTPS session
configuration:

```mermaid
classDiagram
    class Config {
        +Certificates []Certificate
        +ServerName string
        +InsecureSkipVerify bool
        +CipherSuites []string
        +MinVersion uint16
        +MaxVersion uint16
        +PreferServerCipherSuites bool
        +GetCertificate(func(*ClientHelloInfo) (*Certificate, error))
        +VerifyPeerCertificate(func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error)
        +Clone() *Config
    }
    
    class Certificate {
        +Certificate [][]byte
        +PrivateKey crypto.PrivateKey
        +OCSPStaple []byte
        +SignedCertificateTimestamps [][]byte
        +Leaf x509.Certificate
    }
    
    class Conn {
        -conn net.Conn
        -isClient bool
        -config *Config
        -state ConnectionState
        +Handshake() error
        +Read([]byte) (int, error)
        +Write([]byte) (int, error)
        +Close() error
        +ConnectionState() ConnectionState
    }
    
    class ConnectionState {
        +Version uint16
        +HandshakeComplete bool
        +DidResume bool
        +CipherSuite uint16
        +NegotiatedProtocol string
        +ServerName string
        +PeerCertificates []*x509.Certificate
        +VerifiedChains [][]*x509.Certificate
    }
    
    Config "1" o-- "*" Certificate : contains
    Conn "1" --> "1" Config : uses
    Conn "1" --> "1" ConnectionState : maintains
```
