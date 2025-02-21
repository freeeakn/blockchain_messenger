# 🌌 AetherWave Blockchain Messenger Network

A secure and decentralized peer-to-peer messaging system built on blockchain technology. Send encrypted messages across a distributed network while maintaining transparency and integrity through blockchain verification.

## ✨ Features

This project demonstrates a basic blockchain-based messaging system with the following features:

- Distributed blockchain for message storage
- Encrypted messaging using AES-256
- Peer-to-peer network with gossip-based peer discovery
- Proof-of-work consensus mechanism

The system is split into four main components:

- `blockchain.go`: Core blockchain implementation
- `crypto.go`: Encryption/decryption utilities
- `node.go`: P2P networking and peer discovery
- `main.go`: Application entry point and demonstration

## 🚀 Getting Started

### Prerequisites

- Go 1.18 or higher
- Basic understanding of blockchain concepts
- Basic knowledge of Go programming

## Installation

1. Clone the repository:

```bash
git clone git@github.com:freeeakn/AetherWave.git
cd AetherWave
```

2. Ensure you have Go installed:

```bash
go version
```

3. Run the application:

```bash
make
```

## 🖥️ Usage

The current implementation runs a simple demonstration:

Creates a blockchain
Adds sample encrypted messages between "Danya" and "Asur"
Prints the blockchain contents and decrypted messages
Starts a P2P network with three nodes

```bash
go run ./src -address="localhost:port" -name="Danya"
```

```bash
go run ./src -address="localhost:port" -peer="<ip of another peer>:port" -name="Asur" -key="KeyOfAnotherPeer"
```

## Project structure

  ```bash
  src/
  ├── blockchain.go    # Blockchain implementation
  ├── crypto.go        # Encryption utilities
  ├── node.go          # Networking and peer discovery
  └── main.go          # Main application
  README.md            # This file
  ```

## Features

### Blockchain

  1. Stores messages as transactions
  2. Implements proof-of-work mining
  3. Verifies chain integrity

### Encryption

  1. AES-256 encryption for messages
  2. Secure key generation

### Networking

  1. TCP-based P2P communication
  2. Gossip-based peer discovery
  3. Periodic peer list broadcasting

## Extending the Project

 Potential improvements:

 1. Add persistent storage
 2. Implement proper key management
 3. Add user authentication
 4. Enhance consensus mechanism
 5. Add message broadcasting
 6. Implement a user interface
 7. Error handling
 8. Message validation
 9. Proper network synchronization
 10. Message threading

## 🤝 Contributing

Fork the repository
Create a feature branch
Submit a pull request with your changes

## 📜 License

This project is licensed under the MIT License - see the [LICENSE](./LICENSE) file for details.

## 📡 From AetherWave team with love

[freeeakn](https://github.com/freeeakn)
[Routybor](https://github.com/Routybor)

![AetherWaveLogo](./img/logo.svg)
