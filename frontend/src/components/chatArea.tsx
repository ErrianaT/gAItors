import React, { useState } from "react";
import "./chatArea.css";
import botAvatar from "../assets/gator.png";

type Message = {
  sender: "user" | "bot";
  text: string;
};

// Helper Component for the Stars
const StarRating: React.FC<{ rating: number }> = ({ rating }) => {
  return (
    <div className="star-wrapper">
      {[...Array(5)].map((_, i) => {
        const starIndex = i + 1;
        let fillPercentage = 0;

        if (starIndex <= Math.floor(rating)) {
          fillPercentage = 100; // Full star
        } else if (starIndex === Math.ceil(rating)) {
          fillPercentage = (rating % 1) * 100; // Partial star (e.g., 0.7 * 100 = 70%)
        }

        return (
          <span key={i} className="star-container">
            {/* The background (empty) star */}
            <span className="star-empty">★</span>
            {/* The foreground (filled) star with dynamic width */}
            <span 
              className="star-filled" 
              style={{ width: `${fillPercentage}%` }}
            >
              ★
            </span>
          </span>
        );
      })}
      <span className="rating-number">{rating.toFixed(1)}/5</span>
    </div>
  );
};

interface ChatAreaProps {
  messages: Message[];
  onSendMessage: (messages: Message[]) => void;
}

const ChatArea: React.FC<ChatAreaProps> = ({ messages, onSendMessage }) => {
  const [message, setMessage] = useState("");
  const [isLoading, setIsLoading] = useState(false);

  const handleSend = async () => {
    if (!message.trim() || isLoading) return;
    const userMessage: Message = { sender: "user", text: message };
    const initialMessages = [...messages, userMessage];
    onSendMessage(initialMessages);
    setMessage("");
    setIsLoading(true);

    try {
      const response = await fetch("http://localhost:8080/chat", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ role: "user", content: userMessage.text }),
      });
      const data = await response.json();
      const botMessage: Message = {
        sender: "bot",
        text: data.content || "I couldn't find that professor.",
      };
      onSendMessage([...initialMessages, botMessage]);
    } catch (error) {
      onSendMessage([...initialMessages, { sender: "bot", text: "Backend Error!" }]);
    } finally {
      setIsLoading(false);
    }
  };

  // Helper to format the "weird" text into a clean UI
  const renderMessageContent = (text: string) => {
    // 1. Extract the rating (e.g., "3.8")
    const ratingMatch = text.match(/quality rating of (\d\.\d)/);
    const rating = ratingMatch ? parseFloat(ratingMatch[1]) : null;

    // 2. Extract Professor Name (Assuming it starts with "Professor [Name]")
    const nameMatch = text.match(/Professor ([\w\s]+) has/);
    const profName = nameMatch ? nameMatch[1] : null;

    // 3. Clean up the text: Remove the messy headers and bullet points
    // This turns the response into one clean paragraph
    const cleanedText = text
      .replace(/Professor [\w\s]+ has a quality rating of \d\.\d out of 5\./, "")
      .replace(/Student feedback highlights:/g, "")
      .replace(/- /g, " ") // Removes bullet dashes
      .replace(/\n/g, " ") // Removes forced line breaks
      .trim();

    return (
      <div className="formatted-bot-msg">
        {profName && <h3 className="prof-title">Professor {profName}</h3>}
        {rating && <StarRating rating={rating} />}
        <p className="prof-description">{cleanedText}</p>
      </div>
    );
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
              {msg.sender === "bot" && <img src={botAvatar} alt="Bot" className="bot-avatar" />}
              <div className={`message-bubble ${msg.sender}`}>
                {msg.sender === "bot" ? renderMessageContent(msg.text) : msg.text}
              </div>
            </div>
          ))}
          {isLoading && (
            <div className="message-row bot">
              <img src={botAvatar} alt="Bot" className="bot-avatar" />
              <div className="message-bubble bot typing-dots">Gator is thinking...</div>
            </div>
          )}
        </div>
      </div>

      <div className="chat-box">
      <textarea
        className="chat-input"
        placeholder={isLoading ? "Gator is thinking..." : "Ask me anything..."}
        value={message}
        disabled={isLoading}
        onChange={(e) => setMessage(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && !e.shiftKey) {
            e.preventDefault();
            handleSend();
          }
        }}
      />
        <button className="send-button" onClick={handleSend}>Send</button>
      </div>
    </div>
  );
};

export default ChatArea;