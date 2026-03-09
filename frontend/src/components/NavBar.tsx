import React, { useState, useRef, useEffect } from "react";
import "./NavBar.css";
import profileIcon from "../assets/profileIcon.png";
import menuIcon from "../assets/menuIcon.png";

interface NavBarProps {
  isSidePanelOpen: boolean;
  onMenuClick: () => void;
  onSettingsClick: () => void;
}

const NavBar: React.FC<NavBarProps> = ({ onMenuClick, onSettingsClick }) => {
  const [isOpen, setIsOpen] = useState(false);
  
  const menuRef = useRef<HTMLDivElement>(null);

  const toggleMenu = () => {
    setIsOpen((prev) => !prev);
  };

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };

    document.addEventListener("mousedown", handleClickOutside);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, []);

  return (
    <nav className="navbar">
      <div className="navbar-left">
        <img
          src={menuIcon}
          alt="Menu"
          className="menu-icon"
          onClick={onMenuClick}
        />
        <div className="navbar-logo">One-Stop AI Assistant</div>
      </div>

      <div className="navbar-profile" ref={menuRef}>
        <button type="button" className="profile-button" onClick={toggleMenu}>
          <img src={profileIcon} alt="Profile" />
        </button>

        {isOpen && (
          <div className="profile-dropdown">
            <button>Profile</button>
            <button onClick={onSettingsClick}>Settings</button>
            <button>Logout</button>
          </div>
        )}
      </div>
    </nav>
  );
};

export default NavBar;