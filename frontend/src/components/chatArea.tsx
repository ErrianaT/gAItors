import React, { useState, useEffect } from "react";
import "./chatArea.css";
import "leaflet/dist/leaflet.css";
import botAvatar from "../assets/gator.png";
import sunnyIcon from "../assets/sunny.png"
import cloudyIcon from "../assets/cloud.png"
import rainyIcon from "../assets/rain.png"
import partlyCloudyIcon from "../assets/partlycloudy.png"
import { MapContainer, TileLayer, Marker, Popup, useMap } from 'react-leaflet';
import L from 'leaflet';
// Fix for default marker icons not showing up in Webpack/Vite
import markerIcon from 'leaflet/dist/images/marker-icon.png';
import markerShadow from 'leaflet/dist/images/marker-shadow.png';


type Message = {
  sender: "user" | "bot";
  text: string;
};

// Helper for displaying star ratings
const StarRating: React.FC<{ rating: number }> = ({ rating }) => {
  return (
    <div className="star-wrapper">
      {[...Array(5)].map((_, i) => {
        const starIndex = i + 1;
        let fillPercentage = 0;

        if (starIndex <= Math.floor(rating)) {
          fillPercentage = 100;
        } else if (starIndex === Math.ceil(rating)) {
          fillPercentage = (rating % 1) * 100; 
        }

        return (
          <span key={i} className="star-container">
            <span className="star-empty">★</span>
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

const DefaultIcon = L.icon({
    iconUrl: markerIcon,
    shadowUrl: markerShadow,
    iconSize: [25, 41],
    iconAnchor: [12, 41]
});
L.Marker.prototype.options.icon = DefaultIcon;

// Helper to move the map view when coordinates change
const ChangeView: React.FC<{ center: [number, number] }> = ({ center }) => {
  const map = useMap();
  map.setView(center, 16);
  return null;
};

const LeafletMap: React.FC<{ locationName: string }> = ({ locationName }) => {
  const [position, setPosition] = useState<[number, number]>([29.6465, -82.3477]); // Default UF Center

  useEffect(() => {
    const fetchCoords = async () => {
      try {
        // add "University of Florida" to the query to ensure we get the right campus spot
        const query = encodeURIComponent(`${locationName}, University of Florida, Gainesville, FL`);
        const response = await fetch(
          `https://nominatim.openstreetmap.org/search?format=json&q=${query}&limit=1`
        );
        const data = await response.json();

        if (data && data.length > 0) {
          const lat = parseFloat(data[0].lat);
          const lon = parseFloat(data[0].lon);
          setPosition([lat, lon]);
        }
      } catch (error) {
        console.error("Geocoding error:", error);
      }
    };

    if (locationName) fetchCoords();
  }, [locationName]);

  return (
    <div className="leaflet-map-wrapper">
      <MapContainer 
        center={position} 
        zoom={16} 
        scrollWheelZoom={false} 
        style={{ height: '100%', width: '100%' }}
      >
        <TileLayer
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
          attribution='&copy; OpenStreetMap'
        />
        <Marker position={position}>
          <Popup>{locationName}</Popup>
        </Marker>
        <ChangeView center={position} />
      </MapContainer>
    </div>
  );
};
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

  const renderMessageContent = (text: string) => {
    // --- 1. Blue Phone / Safety Logic ---
    if (text.toLowerCase().includes("blue phone")) {
      const locationMatch = text.match(/at ([\w\s]+), located/i);
      const locationName = locationMatch ? locationMatch[1] : "Blue Phone Location";
      const [header, rest] = text.split("**Directions:**");
      const [directions, footer] = rest ? rest.split("**Total distance:**") : ["", ""];

      return (
        <div className="safety-card">
          <h3 className="bold-title">🐊 Gator Safety: Blue Phone</h3>
          
          {/* Our new Leaflet Map component */}
          <LeafletMap locationName={locationName} />
          
          <div className="directions-section">
            <p className="location-highlight">📍 {header.trim()}</p>
            
            <details className="directions-dropdown">
              <summary>View Navigation Steps</summary>
              <div className="directions-list">
                {directions.trim()}
                <div className="travel-stats">
                  <br/>
                  <strong>Total Distance:</strong> {footer.split("**Estimated")[0]}
                  <br/>
                  <strong>Time:</strong> {footer.split("~")[1]}
                </div>
              </div>
            </details>

            {/* Emergency Call Button */}
            <a href="tel:3523921111" className="emergency-button">
              📞 Call UFPD (352-392-1111)
            </a>
          </div>
        </div>
      );
    }
    // --- 2. Weather Logic ---
    if (text.toLowerCase().includes("weather") || text.includes("°F")) {
      // Handles "Midnight/Noon" OR "12:00 PM"
      const forecastRegex = /(Midnight|Noon|\d{1,2}:\d{2}\s[APM]{2}):\s([^,]+),\s(\d+\.\d+°F),.*?(?:humidity\s)?(\d+%)?/gi;
      const matches = [...text.matchAll(forecastRegex)];
    
      if (matches.length > 0) {
        return (
          <div className="weather-forecast">
            <h3 className="bold-title">Gainesville Forecast</h3>
            <div className="forecast-strip">
              {matches.map((match, i) => {
                const iconSrc = getWeatherIcon(match[2]);
                return (
                  <div key={i} className="weather-card">
                    <span className="weather-time">{match[1]}</span>
                    <img src={iconSrc} alt={match[2]} className="weather-icon-png" />
                    <span className="weather-temp">{match[3]}</span>
                    <span className="weather-desc">{match[2]}</span>
                  </div>
                );
              })}
            </div>
            <p className="weather-note">
              {text.split(/Expect|Please note/i)[1] ? `Note: ${text.split(/Expect|Please note/i)[1]}` : ""}
            </p>
          </div>
        );
      }
    }
  
    // --- 3. Professor Logic ---
    const ratingMatch = text.match(/quality rating of (\d\.\d)/);
    const nameMatch = text.match(/Professor ([\w\s]+) has/);
  
    if (ratingMatch || nameMatch) {
      const rating = ratingMatch ? parseFloat(ratingMatch[1]) : null;
      const profName = nameMatch ? nameMatch[1] : null;
  
      const cleanedText = text
        .replace(/Professor [\w\s]+ has a quality rating of \d\.\d out of 5\./, "")
        .replace(/Student feedback highlights:/g, "")
        .replace(/- /g, " ")
        .replace(/\n/g, " ")
        .trim();
  
      return (
        <div className="formatted-bot-msg">
          {profName && <h3 className="bold-title">Professor {profName}</h3>}
          {rating !== null && <StarRating rating={rating} />}
          <p className="prof-description">{cleanedText}</p>
        </div>
      );
    }

    // --- 4. Restaurants Logic ---
    if (text.includes("Rating:") || text.includes("Address:")) {
      // Split the text into blocks based on where a bolded name "**Name**" appears
      const blocks = text.split(/\n(?=\d+\.|\s*-?\s*\*\*)/g);
      const items: any[] = [];

      blocks.forEach(block => {
        // 1. Extract Name: Find the FIRST bolded string, then strip leading dashes/numbers
        const nameMatch = block.match(/\*\*(.*?)\*\*/);
        if (!nameMatch) return;
        
        // Clean name: removes leading "- ", "1. ", and trailing " - Indian Restaurant"
        let cleanName = nameMatch[1]
          .replace(/^[\s\d.-]+/, '') 
          .split(" - ")[0]
          .trim();
      
        // 2. Extract Rating: Improved to handle "**Rating**:" or "Rating:"
        const ratingMatch = block.match(/Rating[:\s*]*([\d.]+)/i);
        
        // 3. Extract Price: Handles "Price Level" or just "Price"
        const priceMatch = block.match(/Price(?:[\s\w]*|)[:\s*]*(\w+)/i);
        
        // 4. Extract Address: Improved to stop at the end of the line or "USA"
        const addressMatch = block.match(/Address[:\s*]*([^|\n\r]+)/i);
      
        if (cleanName && ratingMatch) {
          items.push({
            name: cleanName,
            rating: parseFloat(ratingMatch[1]),
            price: priceMatch ? priceMatch[1] : null,
            address: addressMatch ? addressMatch[1].replace(/,?\s*USA/gi, "").trim() : null
          });
        }
      });

      if (items.length > 0) {
        return (
          <div className="places-container">
            <h3 className="bold-title">📍 Nearby Recommendations</h3>
            <div className="places-simple-list">
              {items.map((item, i) => (
                <div key={i} className="place-item-simple">
                  <div className="place-header-row">
                    <span className="place-name">{item.name}</span>
                    {item.price && (
                      <span className="place-price-tag">
                        {item.price.toLowerCase().includes("moderate") ? "$$" : "$"}
                      </span>
                    )}
                  </div>
                  <div className="place-details-row">
                    <StarRating rating={item.rating} />
                    {item.address && (
                      <span className="place-address-text">• {item.address}</span>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        );
      }
    }
  
    // --- 5. Default Fallback ---
    // If it's neither weather nor a professor, just show the text normally
    return <p className="standard-text">{text}</p>;
  };

  // Helper for displaying weather icons
  const getWeatherIcon = (condition: string) => {
    const cond = condition.toLowerCase();
    
    if (cond.includes("clear") || cond.includes("clearing")) return sunnyIcon;
    if (cond.includes("rain") || cond.includes("drizzle")) return rainyIcon;
    if (cond.includes("broken") || cond.includes("scattered") || cond.includes("clouds")) {
        return cond.includes("broken") ? cloudyIcon : partlyCloudyIcon;
    }
    
    return partlyCloudyIcon;
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