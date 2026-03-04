import { useState } from "react";
import NavBar from "./components/NavBar";
import SidePanel from "./components/SidePanel";
import ChatArea from "./components/chatArea";
import "./Home.css";

function Home() {
  const [isSidePanelOpen, setIsSidePanelOpen] = useState(false);
  const toggleSidePanel = () => setIsSidePanelOpen(prev => !prev);
  
  return (
    <div className="app-container">
      <NavBar onMenuClick={toggleSidePanel} />
      <SidePanel isOpen={isSidePanelOpen} onClose={() => setIsSidePanelOpen(false)}
/>
      <div className="chat-wrapper">
        <ChatArea />
      </div>
    </div>
  );
}

export default Home;
