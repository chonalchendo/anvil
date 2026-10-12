import { useHome } from "../lib/api";
import { HomeView } from "../components/home/HomeView";
import { QueryStatus } from "../components/ui/QueryStatus";

export function HomePage() {
  const { data, error } = useHome();
  return data ? <HomeView home={data} /> : <QueryStatus error={error} what="what is moving" />;
}

