import React, { useState } from "react";
import "./chatArea.css";

const ChatArea: React.FC = () => {
  const [message, setMessage] = useState("");
  const [hasSent, setHasSent] = useState(false);

  const handleSend = () => {
    if (!message.trim()) return;

    console.log("Sending:", message);
    setHasSent(true);     
    setMessage("");
  };

  return (
    <div className="chat-container">
      <h1 className={`title ${hasSent ? "fade-out" : ""}`}>
        Welcome to One-Stop!
      </h1>

      <div className="chat-box">
        <textarea
          className="chat-input"
          placeholder="Ask me anything..."
          value={message}
          onChange={(e) => setMessage(e.target.value)}
        />
        <button className="send-button" onClick={handleSend}>
          Send
        </button>
      </div>
    </div>
  );
};

export default ChatArea;