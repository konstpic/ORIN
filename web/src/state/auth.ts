import { create } from "zustand";

interface AuthState {
  token: string | null;
  setToken: (t: string | null) => void;
}

const STORAGE_KEY = "k8sui-token";

export const useAuth = create<AuthState>((set) => ({
  token: localStorage.getItem(STORAGE_KEY),
  setToken: (t) => {
    if (t) localStorage.setItem(STORAGE_KEY, t);
    else localStorage.removeItem(STORAGE_KEY);
    set({ token: t });
  },
}));
