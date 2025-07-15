// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/security/ReentrancyGuard.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

contract TokenSale is ReentrancyGuard, Ownable {
    IERC20 public usdtToken;
    IERC20 public kaatingaToken;

    uint256 public rate;
    uint256 public hardCap;
    uint256 public totalSold;
    uint256 public saleStart;
    uint256 public saleEnd;
    bool public paused = false;

    mapping(address => uint256) public purchases;

    event TokensPurchased(address indexed buyer, uint256 usdtAmount, uint256 kaatingaAmount);
    event SaleConfigured(uint256 rate, uint256 hardCap, uint256 saleStart, uint256 saleEnd);
    event USDTWithdrawn(address indexed owner, uint256 amount);
    event UnsoldTokensWithdrawn(address indexed owner, uint256 amount);
    event SalePaused(uint256 timestamp);
    event SaleResumed(uint256 timestamp);

    constructor(address _usdtTokenAddress, address _kaatingaTokenAddress) {
        require(_usdtTokenAddress != address(0), "Invalid USDT address");
        require(_kaatingaTokenAddress != address(0), "Invalid KAATINGA address");
        usdtToken = IERC20(_usdtTokenAddress);
        kaatingaToken = IERC20(_kaatingaTokenAddress);
    }

    function configureSale(
        uint256 _rate,
        uint256 _hardCap,
        uint256 _saleStart,
        uint256 _saleEnd
    ) external onlyOwner {
        require(_rate > 0, "Rate must be greater than 0");
        require(_hardCap > 0, "Hard cap must be greater than 0");
        require(_saleStart < _saleEnd, "Invalid sale period");

        rate = _rate;
        hardCap = _hardCap;
        saleStart = _saleStart;
        saleEnd = _saleEnd;

        emit SaleConfigured(_rate, _hardCap, _saleStart, _saleEnd);
    }

    function buyTokens(uint256 _usdtAmount) external nonReentrant {
        require(!paused, "Sale is paused");
        require(block.timestamp >= saleStart, "Sale not started");
        require(block.timestamp <= saleEnd, "Sale ended");
        require(_usdtAmount > 0, "Amount must be greater than 0");

        uint256 kaatingaAmount = (_usdtAmount * rate * 10 ** 18) / 10 ** 6;
        require(totalSold + kaatingaAmount <= hardCap, "Would exceed hard cap");

        require(
            usdtToken.transferFrom(msg.sender, address(this), _usdtAmount),
            "USDT transfer failed"
        );
        require(
            kaatingaToken.transfer(msg.sender, kaatingaAmount),
            "KAATINGA transfer failed"
        );

        purchases[msg.sender] += kaatingaAmount;
        totalSold += kaatingaAmount;

        emit TokensPurchased(msg.sender, _usdtAmount, kaatingaAmount);
    }

    function withdrawUSDT() external onlyOwner {
        uint256 balance = usdtToken.balanceOf(address(this));
        require(balance > 0, "No USDT to withdraw");
        require(usdtToken.transfer(owner(), balance), "USDT transfer failed"); // Add require
        emit USDTWithdrawn(owner(), balance); // Add emit
    }

    function withdrawUnsoldTokens() external onlyOwner {
        require(block.timestamp > saleEnd, "Sale not ended");
        uint256 unsoldTokens = kaatingaToken.balanceOf(address(this));
        require(unsoldTokens > 0, "No tokens to withdraw");
        require(kaatingaToken.transfer(owner(), unsoldTokens), "Withdrawal failed");
        emit UnsoldTokensWithdrawn(owner(), unsoldTokens); // Add this line
    }

    function emergencyPause() external onlyOwner {
        paused = true;
        emit SalePaused(block.timestamp);
    }

    function resumeSale() external onlyOwner {
        require(paused, "Sale is not paused");
        paused = false;
        emit SaleResumed(block.timestamp);
    }

    function getSaleInfo() external view returns (
        uint256 _rate,
        uint256 _hardCap,
        uint256 _totalSold,
        uint256 _saleStart,
        uint256 _saleEnd,
        bool _isActive
    ) {
        return (rate, hardCap, totalSold, saleStart, saleEnd,
            block.timestamp >= saleStart && block.timestamp <= saleEnd);
    }
}
