import { useState } from "react";
import NavBar from "./components/NavBar";
import SidePanel from "./components/SidePanel";
import ChatArea from "./components/chatArea";
import type { Chat, Message } from "./components/types"; // shared types
import "./Home.css";

function Home() {
  const [isSidePanelOpen, setIsSidePanelOpen] = useState(false);
  const [chats, setChats] = useState<Chat[]>([]);
  const [activeChatId, setActiveChatId] = useState<string | null>(null);

  const toggleSidePanel = () => setIsSidePanelOpen(prev => !prev);

  const createNewChat = () => {
    const newChat: Chat = {
      id: crypto.randomUUID(),
      title: `Chat ${chats.length + 1}`,
      messages: [],
    };

    setChats((prev) => [...prev, newChat]);
    setActiveChatId(newChat.id);
  };

  const updateMessages = (newMessages: Message[]) => {
    if (!activeChatId) {
      const newChat: Chat = {
        id: crypto.randomUUID(),
        title: `Chat 1`,
        messages: newMessages,
      };
      setChats([newChat]);
      setActiveChatId(newChat.id);
      return;
    }

    setChats((prev) =>
      prev.map((chat) =>
        chat.id === activeChatId
          ? { ...chat, messages: newMessages }
          : chat
      )
    );
  };

  const activeChat = chats.find(chat => chat.id === activeChatId);

  return (
    <div className="app-container">
      <NavBar onMenuClick={toggleSidePanel} />

      <SidePanel
        isOpen={isSidePanelOpen}
        onClose={() => setIsSidePanelOpen(false)}
        chats={chats}
        activeChatId={activeChatId} 
        onSelectChat={setActiveChatId}
        onNewChat={createNewChat}
      />

      <div className="chat-wrapper">
        <ChatArea
          messages={activeChat?.messages || []}
          onSendMessage={updateMessages}
        />
      </div>
    </div>
  );
}

export default Home;