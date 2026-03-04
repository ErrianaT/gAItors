import React, { useEffect, useRef } from "react";
import "./SidePanel.css";

import messagesIcon from "../assets/messages.png";
import searchIcon from "../assets/search.png";
import settingsIcon from "../assets/settingsIcon.png";

interface SidePanelProps {
  isOpen: boolean;
  width?: number;
  onClose: () => void;
}

const SidePanel: React.FC<SidePanelProps> = ({ isOpen, width = 200, onClose }) => {
  const panelRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        isOpen &&
        panelRef.current &&
        !panelRef.current.contains(event.target as Node)
      ) {
        onClose(); // 👈 close if clicked outside
      }
    };

    document.addEventListener("mousedown", handleClickOutside);

    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, [isOpen, onClose]);

  return (
    <div
      ref={panelRef}
      className={`side-panel ${isOpen ? "open" : ""}`}
      style={{ width: `${width}px` }}
    >
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