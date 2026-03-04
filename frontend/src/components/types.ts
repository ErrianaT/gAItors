export type Message = {
    sender: "user" | "bot"; 
    text: string;
  };

export type Chat = {
    id: string;
    title: string;
    messages: Message[];
  };