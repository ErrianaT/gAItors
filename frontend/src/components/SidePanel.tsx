import { useEffect, useRef, useState } from "react";
import "./SidePanel.css";
import type { Chat } from "./types";
import messagesIcon from "../assets/messages.png";
import searchIcon from "../assets/search.png";
import settingsIcon from "../assets/settingsIcon.png";

interface SidePanelProps {
  isOpen: boolean;
  onClose: () => void;  
  width?: number;
  chats: Chat[];
  onSelectChat: (id: string) => void;
  onNewChat: () => void;
  activeChatId: string | null;
}

const MAX_VISIBLE_CHATS = 8;

const SidePanel: React.FC<SidePanelProps> = ({
  isOpen,
  onClose,    
  width = 200,
  chats,
  onSelectChat,
  onNewChat,
  activeChatId,
}) => {
  const [showAllChats, setShowAllChats] = useState(false);
  const panelRef = useRef<HTMLDivElement>(null);

  // ✅ CLICK OUTSIDE LOGIC
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        isOpen &&
        panelRef.current &&
        !panelRef.current.contains(event.target as Node)
      ) {
        onClose();
      }
    };

    document.addEventListener("mousedown", handleClickOutside);

    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, [isOpen, onClose]);

  const visibleChats = showAllChats
    ? chats
    : chats.slice(0, MAX_VISIBLE_CHATS);

  const shouldScroll = chats.length > MAX_VISIBLE_CHATS && showAllChats;

  return (
    <div
      ref={panelRef}
      className={`side-panel ${isOpen ? "open" : ""}`}
      style={{ width: `${width}px` }}
    >
      <div className="side-panel-content">

        {/* Top fixed items */}
        <ul className="side-panel-list top-list">
          <li onClick={onNewChat}>
            <img src={messagesIcon} alt="New Chat" className="list-icon" />
            <span>New Chat</span>
          </li>
          <li>
            <img src={searchIcon} alt="Search Chats" className="list-icon" />
            <span>Search Chats</span>
          </li>
        </ul>

        {/* Chat Section */}
        <div className="chat-section">
          <div className="your-chats-label">Your Chats</div>

          <div
            className={`chat-list-container ${
              shouldScroll ? "scrollable" : ""
            }`}
          >
            <ul className="side-panel-list">
              {visibleChats.length === 0 && (
                <li className="chat-item">No chats yet</li>
              )}

              {visibleChats.map((chat) => (
                <li
                  key={chat.id}
                  className={`chat-item ${
                    chat.id === activeChatId ? "active" : ""
                  }`}
                  onClick={() => onSelectChat(chat.id)}
                >
                  {chat.title}
                </li>
              ))}

              {chats.length > MAX_VISIBLE_CHATS && (
                <li
                  className="chat-item more-item"
                  onClick={() =>
                    setShowAllChats((prev) => !prev)
                  }
                >
                  {showAllChats
                    ? "Show Less ↑"
                    : `More → (${chats.length - MAX_VISIBLE_CHATS})`}
                </li>
              )}
            </ul>
          </div>
        </div>

        {/* Bottom fixed */}
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