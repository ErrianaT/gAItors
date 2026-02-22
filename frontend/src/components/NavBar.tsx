import React from "react";
import "./NavBar.css";
import profileIcon from "../assets/profileIcon.png";
import menuIcon from "../assets/menuIcon.png";

interface NavBarProps {
  onMenuClick: () => void;
}

const NavBar: React.FC<NavBarProps> = ({ onMenuClick }) => {
  return (
    <nav className="navbar">
      <div className="navbar-left">
        <img src={menuIcon} alt="Menu" className="menu-icon" onClick={onMenuClick}/>
        <div className="navbar-logo">One-Stop AI Assistant</div>
      </div>
      <div className="navbar-profile">
        <img src={profileIcon} alt="Profile" />
      </div>
    </nav>
  );
};

export default NavBar;