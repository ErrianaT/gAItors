import React, { useState, useEffect, useRef } from "react";
import "./chatArea.css";
import "leaflet/dist/leaflet.css";
import botAvatar from "../assets/gator.png";
import uploadIcon from "../assets/upload.png"
import sunnyIcon from "../assets/sunny.png"
import cloudyIcon from "../assets/cloud.png"
import rainyIcon from "../assets/rain.png"
import partlyCloudyIcon from "../assets/partlycloudy.png"
import { MapContainer, TileLayer, Marker, Popup, useMap } from 'react-leaflet';
import L from 'leaflet';
// Fix for default marker icons not showing up in Webpack/Vite
import markerIcon from 'leaflet/dist/images/marker-icon.png';
import markerShadow from 'leaflet/dist/images/marker-shadow.png';
import * as pdfjsLib from "pdfjs-dist";
import mammoth from "mammoth";

pdfjsLib.GlobalWorkerOptions.workerSrc = new URL(
  "pdfjs-dist/build/pdf.worker.min.mjs",
  import.meta.url
).toString();

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

const ChangeView: React.FC<{ center: [number, number] }> = ({ center }) => {
  const map = useMap();
  map.setView(center, 16);
  return null;
};

const LeafletMap: React.FC<{ locationName: string }> = ({ locationName }) => {
  const [position, setPosition] = useState<[number, number]>([29.6465, -82.3477]);

  useEffect(() => {
    if (!locationName) return;
  
    const match =
      Object.keys(UF_LOCATIONS).find(key =>
        locationName.toLowerCase().includes(key.toLowerCase())
      );
  
    if (match) {
      setPosition(UF_LOCATIONS[match]);
    } else {
      console.warn("No UF match found, staying default:", locationName);
    }
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

const parseBotString = (text: string) => {
  const lines = text.split('\n').map(line => line.trim()).filter(Boolean);
  const data: Record<string, string> = {};

  lines.forEach(line => {
    const colonIndex = line.indexOf(':');
    if (colonIndex !== -1) {
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
      return <strong key={index}>{part.replace(/\*\*/g, "")}</strong>;
    }
    return part;
  });
};

const extractPDFText = async (file: File) => {
  const arrayBuffer = await file.arrayBuffer();

  const pdf = await pdfjsLib.getDocument({ data: arrayBuffer }).promise;

  let text = "";

  for (let i = 1; i <= pdf.numPages; i++) {
    const page = await pdf.getPage(i);
    const content = await page.getTextContent();

    const strings = content.items.map((item: any) => item.str);
    text += strings.join(" ") + "\n";
  }

  return text.trim();
};

const extractDocxText = async (file: File) => {
  const arrayBuffer = await file.arrayBuffer();

  const result = await mammoth.extractRawText({
    arrayBuffer
  });

  return result.value; // plain text
};

const extractLocation = (text: string): string => {
  const lines = text.split("\n").map(l => l.trim());

  const candidate = lines.find(line =>
    /at|near|by|location/i.test(line)
  );

  if (candidate) {
    // grabbing text after these words
    const match = candidate.match(/(?:at|near|by)\s+(.*?)(,|\.|$)/i);
    if (match?.[1]) {
      return match[1].trim();
    }
  }
  return lines[0] || "University of Florida";
};

const UF_LOCATIONS: Record<string, [number, number]> = {
  "Holland Law Building": [29.649778662130075, -82.35934158428046],
  "Library West": [29.651454439302427, -82.34290232658918],
  "Marston Science Library": [29.6513, -82.3416],
  "Reitz Union": [29.6465, -82.3477],
  "Hull Road Park & Ride Lot West": [29.63728876970945, -82.36883866371656]
};


const ChatArea: React.FC<ChatAreaProps> = ({ messages, onSendMessage }) => {
  const [message, setMessage] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const handleFileUpload = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) return;
  
    let rawText = "";
  
    try {
      if (file.type === "application/pdf" || file.name.endsWith(".pdf")) {
        rawText = await extractPDFText(file);
      } 
      else if (
        file.name.endsWith(".docx") ||
        file.type.includes("wordprocessingml")
      ) {
        rawText = await extractDocxText(file);
      } 
      else {
        rawText = await file.text();
      }
  
      const backendMessage = `
  I uploaded a document named "${file.name}".
  
  Here is its content:
  
  ${rawText}
  
  Please summarize this document.
  `;
  
      await handleSend(backendMessage, `📎 Uploaded ${file.name}`);
    } catch (err) {
      console.error(err);
      await handleSend(
        "Error reading file. Please try again.",
        "❌ Upload failed"
      );
    }
  
    // reset input so same file can be uploaded again
    event.target.value = "";
  };

  const handleSend = async (
    customMessage?: string,
    displayOverride?: string
  ) => {
    const textToSend = customMessage ?? message;
  
    if (!textToSend.trim() || isLoading) return;
  
    const userMessage: Message = {
      sender: "user",
      text: displayOverride ?? textToSend
    };
  
    const updatedMessages = [...messages, userMessage];
    onSendMessage(updatedMessages);
  
    if (!customMessage) setMessage("");
    setIsLoading(true);
  
    try {
      const response = await fetch("http://localhost:8080/chat", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          role: "user",
          content: textToSend
        }),
      });
  
      const data = await response.json();
  
      let text = data.content || "I've processed your request.";
  
      if (data.images?.[0]?.data) {
        text += `\n\n[IMAGE_DATA]:${data.images[0].data}`;
      }
  
      const botMessage: Message = {
        sender: "bot",
        text: text,
      };
  
      onSendMessage([...updatedMessages, botMessage]);
  
    } catch (error) {
      onSendMessage([
        ...updatedMessages,
        { sender: "bot", text: "Backend Error!" }
      ]);
  
    } finally {
      setIsLoading(false);
    }
  };

  
  const renderMessageContent = (text: string) => {
    const lines = text.split('\n').map(l => l.trim()).filter(Boolean);
   
    // --- RAG tool ---
    const isUploadConfirmation =
    text.toLowerCase().includes("successfully uploaded") ||
    text.toLowerCase().includes("has been successfully uploaded");

    const isRAGDocumentList =
    /fileSearchStores\/.+\/documents/.test(text) &&
    text.includes("displayName:");

    // upload confirmation formatting
    if (isUploadConfirmation) {
    const fileNameMatch = text.match(/"([^"]+\.(pdf|docx|txt|md))"/i);
    const fileName = fileNameMatch?.[1] || "Uploaded file";

    const operationMatch = text.match(/fileSearchStores\/[^\s"]+/);
    const operationId = operationMatch?.[0];

    return (
      <div className="upload-success-card">
        <div className="upload-header">
          <span className="upload-icon">📎</span>
          <h3 className="upload-title">Upload Successful</h3>
        </div>

        <div className="upload-body">
          <p className="upload-file-name">{fileName}</p>

          <p className="upload-status">
            Your document has been added to the file search store.
          </p>

          {operationId && (
            <div className="upload-meta">
              <span className="upload-badge">Added</span>
            </div>
          )}
        </div>
      </div>
    );
    }

    // listing documents formatting
    if (isRAGDocumentList) {
    const formatName = (path: string) => {
      if (!path) return "Untitled";

      // extract filename after /documents/
      const match = path.match(/documents\/(.+)$/);
      const raw = match?.[1] || path;

      return raw
        .replace(/\.pdf|\.docx|\.txt|\.md/gi, "")
        .replace(/-/g, " ")
        .replace(/\s+/g, " ")
        .trim();
    };

    const blocks = text
      .split(/\n(?=- name:)/)
      .filter(b => b.includes("displayName"));

    const docs = blocks.map((block) => {
      const clean = block.replace(/\r/g, "");

      const get = (key: string) => {
        const regex = new RegExp(`-\\s*${key}:\\s*(.+)`, "i");
        const match = clean.match(regex);
        return match?.[1]?.trim();
      };

      const rawName = get("name") || "";

      return {
        name: get("displayName") || formatName(rawName),
        rawName, // (optional debug use)
        size: get("sizeBytes"),
        type: get("mimeType"),
        state: get("state"),
        date: get("createTime"),
      };
    });

    return (
      <div className="file-tool-container">
        <h3 className="bold-title">📁 Course Documents</h3>

        {docs.length === 0 ? (
          <p style={{ color: "orange" }}>
            ⚠️ No documents parsed (check format)
          </p>
        ) : (
          <div className="file-grid">
            {docs.map((doc, i) => (
              <div key={i} className="file-card">
                <div className="file-header">
                  <span className="file-name">{doc.name}</span>
                  <span className="file-badge">{doc.state}</span>
                </div>

                <div className="file-meta">
                  {doc.type && <span>{doc.type}</span>}
                  {doc.size && (
                    <span>
                      {" "}
                      • {(Number(doc.size) / 1024).toFixed(1)} KB
                    </span>
                  )}
                </div>

                {doc.date && (
                  <div className="file-date-group">
                    <div className="file-date">
                      {new Date(doc.date).toLocaleDateString()}
                    </div>
                    <div className="raw-name"><strong>Use the following for deleting a file from file search store:</strong></div>
                    <div className="raw-name">raw name: {doc.rawName}</div>
                  </div>
                )} 
              </div>
            ))}
          </div>
        )}
      </div>
    );
    }
    // --- Professor Tool Formatting ---
    if (text.toLowerCase().includes("quality rating") || text.toLowerCase().includes("professor")) {
      const data = parseBotString(text);
      
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
  
    // --- Restaurants Tool Formatting ---
    if (text.includes("Address:") || text.includes("Rating:") || text.includes("Location:")) {
      const segments = text.split(/\n(?=\d+\.)/);
      const blocks = segments.slice(1);
    
      return (
        <div className="places-container">
          <h3 className="bold-title">📍 Nearby Recommendations</h3>
          <div className="places-simple-list">
            {blocks.map((block, i) => {
              const info = parseBotString(block);
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

    // --- Weather Tool Formatting ---
    if (
      text.includes("°F") ||
      text.toLowerCase().includes("weather")
    ) {
      const lines = text.split("\n").map(l => l.trim()).filter(Boolean);

      const introText = lines[0] || "";

      const lastLine = lines[lines.length - 1] || "";
      const footerText = lastLine.startsWith("-") ? "" : lastLine;

      const weatherData = lines
        .filter(line => /(\d+(\.\d+)?°F)/.test(line))
        .map(line => {
          // --- TIME ---
          const timeMatch = line.match(/\*\*(.*?)\*\*/);
          let time = "";

          if (timeMatch) {
            time = timeMatch[1];
          } else {
            const fallbackMatch = line.match(/-\s*(.*?):/);
            time = fallbackMatch ? fallbackMatch[1] : "";
          }

          time = time
            .replace(/\*\*/g, "")
            .replace(/^-/, "")
            .trim();
          // --- TEMPERATURE ---
          const tempMatch = line.match(/(\d+(\.\d+)?°F)/);
          const temp = tempMatch ? tempMatch[1] : "N/A";

          // --- HUMIDITY ---
          const humidityMatch = line.match(/(\d+%)/);
          const humidity = humidityMatch ? humidityMatch[1] : "";

          // --- CONDITION ---
          let condition = "Clear";

          const afterColon = line.split(":")[1] || "";

          if (afterColon) {
            const cleaned = afterColon
              .replace(/(\d+(\.\d+)?°F)/, "") 
              .replace(/(\d+%)/, "")    
              .replace(/[,|]/g, "")   
              .trim();

            if (cleaned.length > 0) {
              condition = cleaned;
            }
          }

          return {
            time,
            condition,
            temp,
            humidity
          };
        });

      return (
        <div className="weather-forecast">
          <h3 className="bold-title">Weather Forecast</h3>

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
                  <span className="weather-humidity">
                    💧 {slot.humidity}
                  </span>
                )}
              </div>
            ))}
          </div>

          {footerText && (
            <p className="weather-note">{footerText}</p>
          )}
        </div>
      );
    }

    // --- Gym Camera Tool Formatting ---
    if (
      text.toLowerCase().includes("gym") ||
      text.toLowerCase().includes("camera") ||
      text.toLowerCase().includes("crowd")
    ) {
      let detectedLocation = "Camera Feed";
      let imageSrc: string | null = null;
    
      const match = text.match(/\[IMAGE_DATA\]:([\s\S]+)/);
    
      if (match?.[1]) {
        const base64 = match[1].trim();
        imageSrc = `data:image/jpeg;base64,${base64}`;
      }
    
      const displayText = text.split("[IMAGE_DATA]")[0];
    
      return (
        <div className="gym-cam-container">
          <div className="place-header-row">
            <h3 className="bold-title">🏋️ {detectedLocation}</h3>
            <span className="place-price-tag">LIVE</span>
          </div>
    
          <p className="footer-note-text" style={{ margin: "8px 0" }}>
            {displayText.split("!")[0]}!
          </p>
    
          {imageSrc ? (
            <div className="cam-frame">
              <div className="live-badge">● LIVE</div>
              <img src={imageSrc} alt="Gym Camera" className="gym-image" />
              <div className="cam-timestamp">
                {new Date().toLocaleTimeString()}
              </div>
            </div>
          ) : (
            <div className="cam-placeholder">
              <p>Unable to load live image.</p>
            </div>
          )}
        </div>
      );
    }
    
    
    // --- Blue Phone Tool ---
    if (text.toLowerCase().includes("blue safety phone") || text.toLowerCase().includes("nearest blue phone") || text.includes("blue phone")) {
      // const locationMatch = text.match(/at\s+["']?([^"']+)["']?,\s+approximately/i) || text.match(/at the (.*?) \(Building (\d+)\)/i);
      const buildingDisplay = extractLocation(text);
      const cleanLocation = buildingDisplay
        .replace(/\b(NW|NE|SW|SE)\b/gi, "")
        .replace(/\b\d{3,}\b/g, "")
        .replace(/\s+/g, " ")
        .trim();

      const distanceMatch = text.match(/approximately (.*?) away/i) || text.match(/around (.*?) and/i);
      const distance = distanceMatch ? distanceMatch[1] : "Nearby";
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
            <LeafletMap 
              locationName={`${cleanLocation}, Gainesville, FL`} 
              key={cleanLocation}
            />
            </div>
            {directions.length > 0 && (
              <div className="directions-box">
                <span className="loc-label">Directions</span>
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
            <p className="emergency-warning"><strong>Safety First:</strong> Dial <strong>911</strong> for emergencies or <strong>352-392-1111</strong></p>
          </div>
        </div>
      );
    }
  
    return (
      <div className="standard-text">
        {lines.map((line, i) => (
          <p key={i} style={{ marginBottom: '10px', lineHeight: '1.5' }}>{formatBoldText(line)}</p>
        ))}
      </div>
    );
  };

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
        <input
          type="file"
          ref={fileInputRef}
          style={{ display: "none" }}
          accept=".txt,.md,.pdf,.docx,application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document"
          onChange={handleFileUpload}
        />
        <button 
          className="upload-icon-btn" 
          type="button"
          onClick={() => fileInputRef.current?.click()}
          disabled={isLoading}
        >
          <img src={uploadIcon} alt="Upload" className="upload-png" />
          <span className="upload-text">Upload & Summarize</span>
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

        <button 
          className="send-button" 
          onClick={() => handleSend()} 
          disabled={isLoading || !message.trim()}
        >
          Send
        </button>
      </div>
    </div>
  );
};

export default ChatArea;