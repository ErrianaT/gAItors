import React from "react";
import "./SidePanel.css";

import messagesIcon from "../assets/messages.png"
import searchIcon from "../assets/search.png"
import settingsIcon from "../assets/settingsIcon.png"

interface SidePanelProps {
  isOpen: boolean;      
  width?: number;        
}

const SidePanel: React.FC<SidePanelProps> = ({ isOpen, width = 250 }) => {
    return (
      <div className={`side-panel ${isOpen ? "open" : ""}`} style={{ width: `${width}px` }}>
        <div className="side-panel-content">
          <ul className="side-panel-list">
            <li>
              <img src={messagesIcon} alt="New Chat" className="list-icon" />
              <span>New Chat</span>
            </li>
            <li>
              <img src={searchIcon} alt="Search Chats" className="list-icon" />
              <span>Search Chats</span>
            </li>
            <li>
              <span>Your Chats</span>
            </li>
          </ul>
          <ul className="side-panel-list bottom-list">
            <li>
              <img src={settingsIcon} alt="Settings" className="list-icon" />
              <span>Settings</span>
            </li>
          </ul>
        </div>
      </div>
    );
  };
  

export default SidePanel;