import React, { useState } from "react";
import "./chatArea.css";
import botAvatar from "../assets/gator.png";

type Message = {
  sender: "user" | "bot";
  text: string;
};

interface ChatAreaProps {
  messages: Message[];
  onSendMessage: (messages: Message[]) => void;
}


const ChatArea: React.FC<ChatAreaProps> = ({
  messages,
  onSendMessage,
}) => {
  const [message, setMessage] = useState("");

  const handleSend = () => {
    if (!message.trim()) return;

    const userMessage: Message = {
      sender: "user",
      text: message,
    };

    const botMessage: Message = {
      sender: "bot",
      text: "", // placeholder until backend is ready
    };

    const updatedMessages = [
      ...messages,
      userMessage,
      botMessage,
    ];

    onSendMessage(updatedMessages);
    setMessage("");
  };

  const hasSent = messages.length > 0;

  return (
    <div className="chat-container">
      <div className={`welcome-area ${hasSent ? "fade-out" : ""}`}>
        <h1 className="title">Welcome to One-Stop!</h1>
        <img src={botAvatar} alt="Gator Bot" className="welcome-gator" />
      </div>

      <div className="messages-container">
        <div className="messages">
          {messages.map((msg, index) => (
            <div key={index} className={`message-row ${msg.sender}`}>
              {msg.sender === "bot" && (
                <img
                  src={botAvatar}
                  alt="Bot"
                  className="bot-avatar"
                />
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
          onKeyDown={(e) => {
            if(e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              handleSend();
            }
          }
          }
        />
        <button className="send-button" onClick={handleSend}>Send</button>
      </div>
    </div>
  );
};

export default ChatArea;