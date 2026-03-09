import { useState } from "react";
import NavBar from "./components/NavBar";
import SidePanel from "./components/SidePanel";
import ChatArea from "./components/chatArea";
import type { Chat, Message } from "./components/types";
import "./Home.css";

function Home() {
  const [isSidePanelOpen, setIsSidePanelOpen] = useState(false);
  const [chats, setChats] = useState<Chat[]>([]);
  const [activeChatId, setActiveChatId] = useState<string | null>(null);
  const [showSettings, setShowSettings] = useState(false);

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

  const updateChatTitle = (chatId: string, message: string) => {
    setChats((prevChats) =>
      prevChats.map((chat) => {
        if (chat.id === chatId && chat.title.startsWith("Chat")) {
          return {
            ...chat,
            title:
              message.slice(0, 35) +
              (message.length > 35 ? "..." : "")
          };
        }
        return chat;
      })
    );
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
    
    const lastUserMessage = [...newMessages].reverse().find(msg => msg.sender === "user");
  
    if (lastUserMessage) {
      updateChatTitle(activeChatId, lastUserMessage.text);
    }
  
    setChats((prev) =>
      prev.map((chat) =>
        chat.id === activeChatId
          ? { ...chat, messages: newMessages }
          : chat
      )
    );
  };

  const clearChat = () => {
    const newChat: Chat = {
      id: crypto.randomUUID(),
      title: "Chat 1",
      messages: [],
    };
    setChats([newChat]);
    setActiveChatId(newChat.id);
  };

  const activeChat = chats.find(chat => chat.id === activeChatId);

  return (
    <div className="app-container">
      <NavBar
        isSidePanelOpen={isSidePanelOpen}
        onMenuClick={toggleSidePanel}
        onSettingsClick={() => setShowSettings(true)}
      />

      <SidePanel
        isOpen={isSidePanelOpen}
        chats={chats}
        onClose={() => setIsSidePanelOpen(false)} 
        activeChatId={activeChatId} 
        onSelectChat={setActiveChatId}
        onNewChat={createNewChat}
        clearChat={clearChat}
        showSettings={showSettings}       // pass down state
        setShowSettings={setShowSettings}
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