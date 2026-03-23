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

  // Helper to create a fresh chat object
  const createNewChatObject = (messages: Message[] = []) => ({
    id: crypto.randomUUID(),
    title: messages.length > 0 ? messages[0].text.slice(0, 30) : `Chat ${chats.length + 1}`,
    messages: messages,
  });

  const createNewChat = () => {
    const newChat = createNewChatObject();
    setChats((prev) => [...prev, newChat]);
    setActiveChatId(newChat.id);
  };

  const updateChatTitle = (chatId: string, message: string) => {
    setChats((prevChats) =>
      prevChats.map((chat) => {
        // Only update title if it's still the default "Chat X" title
        if (chat.id === chatId && chat.title.startsWith("Chat")) {
          return {
            ...chat,
            title: message.slice(0, 35) + (message.length > 35 ? "..." : "")
          };
        }
        return chat;
      })
    );
  };

  const updateMessages = (newMessages: Message[]) => {
    // FIX: If there's no active chat, create one and add the messages
    if (!activeChatId) {
      const newChat = createNewChatObject(newMessages);
      setChats([newChat]);
      setActiveChatId(newChat.id);
      return;
    }
    
    // Update the title based on the most recent user message
    const lastUserMessage = [...newMessages].reverse().find(msg => msg.sender === "user");
    if (lastUserMessage) {
      updateChatTitle(activeChatId, lastUserMessage.text);
    }
  
    // Update the existing chat's message list
    setChats((prev) =>
      prev.map((chat) =>
        chat.id === activeChatId
          ? { ...chat, messages: newMessages }
          : chat
      )
    );
  };

  const clearChat = () => {
    setChats([]);
    setActiveChatId(null);
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
        showSettings={showSettings}
        setShowSettings={setShowSettings}
      />

      <div className="chat-wrapper">
        <ChatArea
          // If no active chat, we pass an empty array to ChatArea
          messages={activeChat?.messages || []}
          onSendMessage={updateMessages}
        />
      </div>
    </div>
  );
}

export default Home;