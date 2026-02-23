import React, { useState } from "react";
import "./chatArea.css";
import botAvatar from "../assets/gator.png";

type Message = {
  sender: "user" | "bot";
  text: string;
};

const ChatArea: React.FC = () => {
  const [message, setMessage] = useState("");
  const [messages, setMessages] = useState<Message[]>([]);
  const [hasSent, setHasSent] = useState(false);

  const handleSend = () => {
    if (!message.trim()) return;

    const userMessage: Message = {
      sender: "user",
      text: message,
    };

    const botMessage: Message = {
      sender: "bot",
      text: "", // until backend is ready
    };

    setMessages((prev) => [...prev, userMessage, botMessage]);
    setHasSent(true);
    setMessage("");
  };

  return (
    <div className="chat-container">
      {!hasSent && (
        <h1 className="title">Welcome to One-Stop!</h1>
      )}

      <div className="messages-container">
        <div className="messages">
          {messages.map((msg, index) => (
            <div key={index} className={`message-row ${msg.sender}`}>
              {msg.sender === "bot" && (
                <img src={botAvatar} alt="Bot" className="bot-avatar" />
              )}
              <div className={`message-bubble ${msg.sender}`}>
                {msg.text || <div className="bot-placeholder" />}
              </div>
            </div>
          ))}
        </div>
      </div>

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