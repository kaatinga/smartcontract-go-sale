// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

contract KaatingaToken is ERC20, Ownable {
    uint256 public constant MAX_SUPPLY = 1000000 * 10**18; // 1 million tokens

    constructor() ERC20("Kaatinga Token", "KAATINGA") {
        // Mint initial supply to owner (you)
        _mint(msg.sender, MAX_SUPPLY);
    }

    function decimals() public pure override returns (uint8) {
        return 18;
    }
}
