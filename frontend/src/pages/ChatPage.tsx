import { useNavigate, useParams } from "react-router";
import { Inbox } from "../components/Inbox";
import { useWide } from "../lib/chats";

/** /chat and /chat/:id. Phones show the list or one conversation full screen; wide screens show both. */
export function ChatPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const split = useWide();
  const selected = id ? Number(id) : null;
  const select = (next: number | null) => navigate(next === null ? "/chat" : `/chat/${next}`, { replace: split });

  if (!split && selected === null) {
    return (
      <div className="-mx-4 -mt-6 sm:mx-0 sm:mt-0">
        <h1 className="px-4 pt-4 pb-1 font-display text-3xl font-extrabold italic sm:px-0 sm:pt-0">Chat</h1>
        <div className="bg-card sm:overflow-hidden sm:rounded-2xl sm:border sm:border-line">
          <Inbox selected={null} onSelect={select} split={false} />
        </div>
      </div>
    );
  }
  return (
    // One conversation fills the screen under the header, so the composer sits at the bottom like a messaging app.
    <div className="-mx-4 -my-6 flex h-[calc(100dvh-60px)] flex-col overflow-hidden bg-card sm:-my-8 lg:mx-0 lg:my-0 lg:h-[min(44rem,calc(100dvh-8rem))] lg:rounded-2xl lg:border lg:border-line">
      <Inbox selected={selected} onSelect={select} split={split} />
    </div>
  );
}
