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

// helper to extract data 
const parseBotString = (text: string) => {
  const lines = text.split('\n').map(line => line.trim()).filter(Boolean);
  const data: Record<string, string> = {};

  lines.forEach(line => {
    const colonIndex = line.indexOf(':');
    if (colonIndex !== -1) {
      // removeing dashes, stars, dots
      const key = line.slice(0, colonIndex)
        .replace(/^[^a-zA-Z0-9]+/, '') 
        .replace(/[*#-]/g, '')
        .trim()
        .toLowerCase();
        
      const value = line.slice(colonIndex + 1).replace(/\*/g, '').trim();
      data[key] = value;
    }
  });

  if (data['location'] && !data['address']) data['address'] = data['location'];
  
  return data;
};

const formatBoldText = (text: string) => {
  const parts = text.split(/(\*\*.*?\*\*)/g);

  return parts.map((part, index) => {
    if (part.startsWith("**") && part.endsWith("**")) {
      // wrap in <strong></strong> to bold text
      return <strong key={index}>{part.replace(/\*\*/g, "")}</strong>;
    }
    return part;
  });
};

const ChatArea: React.FC<ChatAreaProps> = ({ messages, onSendMessage }) => {
  const [message, setMessage] = useState("");
  const [file, setFile] = useState<File | null>(null); // Track the selected file
  const [isLoading, setIsLoading] = useState(false);
  const fileInputRef = React.useRef<HTMLInputElement>(null); // Ref to trigger the hidden input

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files[0]) {
      setFile(e.target.files[0]);
    }
  };

  const handleSend = async () => {
    if ((!message.trim() && !file) || isLoading) return;

    // If there's a file, we append the name to the user's bubble for clarity
    const userText = file ? `${message} (Attachment: ${file.name})`.trim() : message;
    const userMessage: Message = { sender: "user", text: userText };
    const initialMessages = [...messages, userMessage];
    
    onSendMessage(initialMessages);
    const currentMessage = message; // Store to send to backend
    const currentFile = file;
    
    setMessage("");
    setFile(null); // Reset file state
    setIsLoading(true);

    try {
      // Use FormData for multipart/form-data (required for file uploads)
      const formData = new FormData();
      formData.append("content", currentMessage);
      if (currentFile) {
        formData.append("file", currentFile);
      }

      const response = await fetch("http://localhost:8080/chat", {
        method: "POST",
        // Do NOT set Content-Type header; the browser will set it automatically with the boundary
        body: formData,
      });

      const data = await response.json();
      const botMessage: Message = {
        sender: "bot",
        text: data.content || "I've processed your document.",
      };
      onSendMessage([...initialMessages, botMessage]);
    } catch (error) {
      onSendMessage([...initialMessages, { sender: "bot", text: "Backend Error!" }]);
    } finally {
      setIsLoading(false);
    }
  };

  const renderMessageContent = (text: string) => {
    const lines = text.split('\n').map(l => l.trim()).filter(Boolean);
  
    // --- 1. Professor Tool Formatting ---
    if (text.toLowerCase().includes("quality rating") || text.toLowerCase().includes("professor")) {
      const data = typeof parseBotString === 'function' ? parseBotString(text) : {};
      const lines = text.split('\n').map(l => l.trim()).filter(Boolean);
      
      const rawRating = data['quality rating'] || data['rating'];
      const ratingMatch = text.match(/(\d\.\d)\s*\/\s*5/) || text.match(/rating of (\d\.\d)/i);
      const rating = rawRating 
        ? parseFloat(rawRating.replace(/[^0-9.]/g, '')) 
        : (ratingMatch ? parseFloat(ratingMatch[1]) : 0);

      const takeAgain = data['would take again'] || (text.match(/(\d+%)/)?.[1] || null);

      const difficulty = data['level of difficulty'] || (text.match(/difficulty.*?(\d\.\d)/i)?.[1] || null);
      const name = (data['professor'] || text.match(/Professor:?\s*([A-Za-z\s]+)/i)?.[1] || "Instructor").replace(/\*/g, '');

      const feedbackParagraphs = lines.filter(l => 
        !l.toLowerCase().includes('quality rating') && 
        !l.toLowerCase().includes('would take again') &&
        !l.toLowerCase().includes('difficulty') &&
        !l.startsWith('#')
      );

      return (
        <div className="formatted-bot-msg prof-card">
          <h3 className="bold-title">Professor {name.trim()}</h3>
          <StarRating rating={rating} />
          
          <div className="prof-stats-grid">
            {takeAgain && (
              <div className="stat-pill">
                <span className="stat-label">Retake:</span>
                <span className="stat-value">{takeAgain}</span>
              </div>
            )}
            {difficulty && (
              <div className="stat-pill">
                <span className="stat-label">Difficulty:</span>
                <span className="stat-value">{difficulty}</span>
              </div>
            )}
          </div>

          <div className="prof-content">
            <div className="prof-description">
              {feedbackParagraphs.length > 0 ? (
                feedbackParagraphs.map((p, i) => (
                  <p key={i} className={p.startsWith('-') ? "feedback-bullet" : ""}>
                    {formatBoldText(p.replace(/^[-•]\s*/, ''))}
                  </p>
                ))
              ) : (
                <p>No feedback available.</p>
              )}
            </div>
          </div>
        </div>
      );
    }
  
    // --- 2. Restaurants Tool Formatting ---
    if (text.includes("Address:") || text.includes("Rating:") || text.includes("Location:")) {
      const segments = text.split(/\n(?=\d+\.)/);
      const blocks = segments.slice(1);
    
      return (
        <div className="places-container">
          <h3 className="bold-title">📍 Nearby Recommendations</h3>
          <div className="places-simple-list">
            {blocks.map((block, i) => {
              const info = parseBotString(block);
              
              // removes numbers, dots, and asterices 
              const name = block.split('\n')[0]
                .replace(/^[0-9.\s-]+/, '') 
                .replace(/\*/g, '')         
                .trim();
              
              return (
                <div key={i} className="place-item-simple">
                  <div className="place-header-row">
                    <span className="place-name">{name}</span>
                    <span className="place-price-tag">
                      {info['price']?.toLowerCase().includes("inexpensive") ? "$" : "$$"}
                    </span>
                  </div>
                  <div className="place-details-row">
                    <StarRating rating={parseFloat(info['rating'] || "0")} />
                    {/* matches address or location */}
                    {info['address'] && (
                      <span className="place-address-text">• {info['address']}</span>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      );
    }
    // --- 3. Weather Tool Formatting ---
    if (text.includes("°F") || text.toLowerCase().includes("forecast")) {
      const lines = text.split('\n').map(l => l.trim()).filter(Boolean);
      
      const forecastLines = lines.filter(line => line.startsWith('- **'));
      const introText = lines[0]; 
      const footerText = lines[lines.length - 1].startsWith('-') ? "" : lines[lines.length - 1];
    
      const weatherData = forecastLines.map(line => {
        const timeMatch = line.match(/\*\*(.*?)\*\*/);
        const time = timeMatch ? timeMatch[1].trim() : "";
        const details = line.split('**:')[1] || "";
        
        const parts = details.split(',').map(p => p.trim());
        const condition = parts[0] || "Clear";
        const temp = parts[1] || "N/A";
        
        let humidity = parts[2] || "";
        if (humidity) {
            humidity = humidity.replace(/[a-zA-Z\s]+/g, '').trim(); 
        }
        
        return { time, condition, temp, humidity };
      });
    
      return (
        <div className="weather-forecast">
          <h3 className="bold-title">Gainesville Forecast</h3>
          <p className="weather-intro">{introText}</p>
          
          <div className="forecast-strip">
            {weatherData.map((slot, i) => (
              <div key={i} className="weather-card">
                <span className="weather-time">{slot.time}</span>
                <img 
                  src={getWeatherIcon(slot.condition)} 
                  alt={slot.condition} 
                  className="weather-icon-png" 
                />
                <span className="weather-temp">{slot.temp}</span>
                <span className="weather-desc">{slot.condition}</span>
                {slot.humidity && (
                  <span className="weather-humidity">💧 {slot.humidity}</span>
                )}
              </div>
            ))}
          </div>
          
          {footerText && <p className="weather-note">{footerText}</p>}
        </div>
      );
    }

    // --- 4. RTS Bus Tool Formatting ---
    if (text.includes("Route") || text.includes("Leg")) {
      const lines = text.split('\n').map(l => l.trim()).filter(Boolean);
      
      // extracting bus times 
      const extractTimes = (str: string) => {
        const timeRegex = /(\d{1,2}:\d{2}\s?[APM]{2})/gi;
        return str.match(timeRegex) || [];
      };

      const directRoutes = lines.filter(l => 
        l.startsWith('- **Route') && !l.toLowerCase().includes('via')
      ).map(line => {
        const route = line.match(/\*\*Route (.*?)\*\*/)?.[1] || "";
        const times = extractTimes(line);
        return { 
          route, 
          departs: times[0] || "N/A" 
        };
      });

      const legs = lines.filter(l => l.toLowerCase().includes('leg')).map(line => {
        const times = extractTimes(line);
        const cleanLine = line.replace(/^[-*]\s*/, '').replace(/\*\*/g, '');
        
        return {
          text: cleanLine,
          departs: times[0],
          arrives: times[1]
        };
      });

      return (
        <div className="transit-container">
          <h3 className="bold-title">🚌 Bus Schedule</h3>
          
          {directRoutes.map((bus, i) => (
            <div key={i} className="bus-card">
              <div className="bus-number">#{bus.route}</div>
              <div className="bus-info">
                <span className="bus-time">Departs: {bus.departs}</span>
                <span className="bus-status">Direct Trip</span>
              </div>
            </div>
          ))}

          {legs.length > 0 && (
            <div className="transfer-card">
              <div className="transfer-header">Transfer Details</div>
              <div className="timeline">
                {legs.map((leg, i) => (
                  <div key={i} className="timeline-item">
                    <div className="timeline-dot"></div>
                    <div className="timeline-content">
                      <div className="leg-description">{leg.text}</div>
                      {(leg.departs || leg.arrives) && (
                        <div className="leg-times">
                          {leg.departs && <span><b>Departs:</b> {leg.departs}</span>}
                          {leg.arrives && <span><b>Arrives:</b> {leg.arrives}</span>}
                        </div>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
          
          <p className="footer-note-text">{lines[lines.length - 1]}</p>
        </div>
      );
    }

    type GymId = "swrc_weight1" | "swrc_weight2" | "srfc_weight";

    const GYM_URLS: Record<string, string> = {
      "srfc_weight":   "http://recsports.ufl.edu/cam/cam8.jpg",
      "srfc_cardio":   "http://recsports.ufl.edu/cam/cam7.jpg",
      "swrc_weight1":  "http://recsports.ufl.edu/cam/cam1.jpg",
      "swrc_weight2":  "http://recsports.ufl.edu/cam/cam4.jpg",
      "swrc_cardio":   "http://recsports.ufl.edu/cam/cam5.jpg",
      "swrc_basket12": "http://recsports.ufl.edu/cam/cam3.jpg",
      "swrc_basket34": "http://recsports.ufl.edu/cam/cam2.jpg",
      "swrc_basket56": "http://recsports.ufl.edu/cam/cam6.jpg",
    };

    // --- 5. Gym Camera Tool Formatting ---
    if (text.toLowerCase().includes("gym") || text.toLowerCase().includes("camera")) {
      let imageSrc = null;
      let detectedLocation = "Gym Feed";

      // 1. Checking for Base64 (from backend)
      const base64Match = text.match(/(image\/(?:jpeg|png)\|[A-Za-z0-9+/=]+)/);
      
      if (base64Match) {
        const [mime, data] = base64Match[0].split('|');
        imageSrc = `data:${mime};base64,${data}`;
      } else {
        // 2. Fallback: Identifying which camera the LLM is talking about by checking against the keys in GYM_URLS
        // We check against the keys in GYM_URLS
        const keys = Object.keys(GYM_URLS);
        const foundKey = keys.find(key => 
          text.toLowerCase().includes(key.replace('_', ' ')) || 
          text.toLowerCase().includes(key.split('_')[1]) // e.g., "weight1"
        );

        if (foundKey) {
          imageSrc = GYM_URLS[foundKey];
          detectedLocation = foundKey.replace('_', ' ').toUpperCase();
        }
      }

      return (
        <div className="gym-cam-container">
          <div className="place-header-row">
            <h3 className="bold-title">🏋️ {detectedLocation}</h3>
            <span className="place-price-tag">LIVE</span>
          </div>
          
          <p className="footer-note-text" style={{ margin: '8px 0' }}>
            {text.split('!')[0]}!
          </p>
          
          {imageSrc ? (
            <div className="cam-frame">
              <div className="live-badge">● LIVE STREAM</div>
              <img 
                src={imageSrc} 
                alt="Gym Camera" 
                className="gym-image" 
                loading="lazy"
              />
              <div className="cam-timestamp">
                {new Date().toLocaleTimeString()}
              </div>
            </div>
          ) : (
            <div className="cam-placeholder">
              <p>Unable to load live image. Please check RecSports status.</p>
            </div>
          )}
        </div>
      );
    }

      // --- 7. Blue Phone / Emergency Tool (Adaptive + Map Version) ---
  if (text.toLowerCase().includes("blue safety phone") || text.toLowerCase().includes("blue emergency phone") || text.includes("blue phone")) {
    
    // 1. Capture Location: Handles "Reitz Union North" OR NW Holland Law (Building 757)
    const locationMatch = text.match(/at\s+["']?([^"']+)["']?,\s+approximately/i) || 
                          text.match(/at the (.*?) \(Building (\d+)\)/i);
    
    const buildingDisplay = locationMatch ? locationMatch[1] : "Campus Location";
    
    // 2. Capture Distance/Time
    const distanceMatch = text.match(/approximately (.*?) away/i) || text.match(/around (.*?) and/i);
    const distance = distanceMatch ? distanceMatch[1] : "Nearby";

    // 3. Capture Directions (Look for numbered lines)
    const lines = text.split('\n').map(l => l.trim()).filter(Boolean);
    const directions = lines.filter(l => /^\d+\./.test(l));

    return (
      <div className="emergency-card">
        <div className="emergency-header">
          <span className="emergency-icon">🚨</span>
          <h3 className="emergency-title">Blue Phone Locator</h3>
        </div>
        
        <div className="emergency-body">
          <div className="loc-main">
            <span className="loc-label">Target Location</span>
            <div className="loc-value-row">
              <span className="loc-value">{buildingDisplay}</span>
              <span className="dist-tag">{distance}</span>
            </div>
          </div>

          <div className="mini-map-container" style={{ height: '200px', margin: '12px 0', borderRadius: '8px', overflow: 'hidden' }}>
            <LeafletMap locationName={buildingDisplay} />
          </div>

          {directions.length > 0 && (
            <div className="directions-box">
              <span className="loc-label">Walking Directions</span>
              <div className="steps-list">
                {directions.map((step, i) => (
                  <div key={i} className="step-item">
                    <span className="step-num">{i + 1}</span>
                    <span className="step-text">{formatBoldText(step.replace(/^\d+\.\s*/, ''))}</span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>

        <div className="emergency-footer">
          <p className="emergency-warning">
            <strong>Safety First:</strong> If you are in immediate danger, dial <strong>911</strong> or <strong>352-392-1111</strong>.
          </p>
        </div>
      </div>
    );
  }
  
    // --- #. Fallback & Default Formatting---
    return (
      <div className="standard-text">
        {lines.map((line, i) => (
          <p key={i} style={{ marginBottom: '10px', lineHeight: '1.5' }}>
            {formatBoldText(line)}
          </p>
        ))}
      </div>
    );
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

      {/* NEW: File status indicator just above the box */}
      {file && (
        <div style={{ fontSize: '12px', color: '#0021A5', padding: '0 20px 5px' }}>
          📎 Ready to upload: <strong>{file.name}</strong>
        </div>
      )}

      <div className="chat-box">
        {/* Hidden File Input */}
        <input 
          type="file" 
          ref={fileInputRef} 
          style={{ display: 'none' }} 
          onChange={handleFileChange}
          accept=".pdf,.doc,.docx,.txt"
        />
        
        {/* Attachment Button */}
        <button 
          className="attach-button" 
          onClick={() => fileInputRef.current?.click()}
          title="Upload document"
        >
          🔗
        </button>

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