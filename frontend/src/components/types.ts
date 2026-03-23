export type Message = {
  sender: "user" | "bot";
  text: string;
  role?: string; 
};

export type Chat = {
    id: string;
    title: string;
    messages: Message[];
  };