import React, { useState, useRef, useEffect } from "react";
import "./NavBar.css";
import profileIcon from "../assets/profileIcon.png";
import menuIcon from "../assets/menuIcon.png";

interface NavBarProps {
  onMenuClick: () => void;
  onSettingsClick: () => void;
}

const NavBar: React.FC<NavBarProps> = ({ onMenuClick, onSettingsClick }) => {
  const [isOpen, setIsOpen] = useState(false);
  const [showProfileOverlay, setShowProfileOverlay] = useState(false);
  
  // default name is gator
  const [userName, setUserName] = useState(() => {
    return localStorage.getItem("preferredName") || "Gator";
  });
  const [tempName, setTempName] = useState(userName);

  const menuRef = useRef<HTMLDivElement>(null);

  const toggleMenu = () => setIsOpen((prev) => !prev);

  const handleSaveName = () => {
    setUserName(tempName);
    localStorage.setItem("preferredName", tempName);
    setShowProfileOverlay(false);
  };

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  return (
    <nav className="navbar">
      <div className="navbar-left">
        <img src={menuIcon} alt="Menu" className="menu-icon" onClick={onMenuClick} />
        <div className="navbar-logo">One-Stop AI Assistant</div>
      </div>

      <div className="navbar-profile" ref={menuRef}>
        <button type="button" className="profile-button" onClick={toggleMenu}>
          <img src={profileIcon} alt="Profile" />
        </button>

        {isOpen && (
          <div className="profile-dropdown">
            <div className="dropdown-user-info">Hi, {userName}!</div>
            <button onClick={() => { setShowProfileOverlay(true); setIsOpen(false); }}>Profile</button>
            <button onClick={() => { onSettingsClick(); setIsOpen(false); }}>Settings</button>
          </div>
        )}
      </div>

      {/* Profile Overlay Modal */}
      {showProfileOverlay && (
        <div className="overlay-backdrop">
          <div className="profile-modal">
            <h3>Update Profile</h3>
            <p>What should I call you?</p>
            <input 
              type="text" 
              value={tempName} 
              onChange={(e) => setTempName(e.target.value)}
              placeholder="Enter your name..."
              autoFocus
            />
            <div className="modal-actions">
              <button className="cancel-btn" onClick={() => setShowProfileOverlay(false)}>Cancel</button>
              <button className="save-btn" onClick={handleSaveName}>Save Name</button>
            </div>
          </div>
        </div>
      )}
    </nav>
  );
};

export default NavBar;