import { useState } from "react";
import NavBar from "./components/NavBar";
import SidePanel from "./components/SidePanel";
import "./Home.css";

function Home() {
  const [isSidePanelOpen, setIsSidePanelOpen] = useState(false);
  const toggleSidePanel = () => setIsSidePanelOpen(prev => !prev);
  
  return (
    <div className="app-container">
      <NavBar onMenuClick={toggleSidePanel} />
      <SidePanel isOpen={isSidePanelOpen} />
    </div>
  );
}

export default Home;