# Token Sale System (Solidity + Go)

This project demonstrates a token sale (ICO/IDO) implemented using:

- Solidity smart contracts (ERC-20)
- Go backend service for event handling and integration
- PostgreSQL for tracking purchases

## Kaatinga Token

The Kaatinga Token is an ERC-20 token with the following specifications:
- **Name**: Kaatinga Token
- **Symbol**: KAATINGA
- **Decimals**: 18
- **Max Supply**: 1,000,000 KAATINGA
- **Features**: Ownable, fixed supply minted to deployer

## Structure

```
smart-contracts/   # Solidity contracts using Hardhat
backend/           # Go backend
migrations/        # SQL migrations
```

## Testing

The test suite includes:

- **Anvil local blockchain** (via testcontainers)
- **Smart contract deployment** and interaction
- **Token sale flow testing** (buy, admin functions)
- **PostgreSQL integration** for purchase tracking

### Running Tests

```bash
# Install dependencies
cd smart-contracts && npm install

# Run integration tests
```
