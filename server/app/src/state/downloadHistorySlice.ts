import { createSlice, PayloadAction } from "@reduxjs/toolkit";
import { RootState } from "./store";

export type DownloadHistoryItem = {
  name: string;
  downloadPath?: string;
  timestamp: number;
};

interface DownloadHistoryState {
  items: DownloadHistoryItem[];
}

const loadState = (): DownloadHistoryItem[] => {
  try {
    const items = (JSON.parse(localStorage.getItem("downloads")!) ??
      []) as DownloadHistoryItem[];

    return items.map((item) => {
      const name = item?.name ?? "";
      if (!name.includes("/") && !name.includes("\\")) {
        return item;
      }

      const normalized = name.replace(/\\/g, "/");
      const parts = normalized.split("/").filter((x) => x.length > 0);
      return {
        ...item,
        name: parts.length > 0 ? parts[parts.length - 1] : name
      };
    });
  } catch (err) {
    return [];
  }
};

const initialState: DownloadHistoryState = {
  items: loadState()
};

export const downloadHistorySlice = createSlice({
  name: "downloadHistory",
  initialState,
  reducers: {
    addDownloadHistoryItem: (
      state,
      action: PayloadAction<DownloadHistoryItem>
    ) => {
      state.items = [action.payload, ...state.items].slice(0, 64);
    },
    deleteDownloadHistoryItem: (state, action: PayloadAction<number>) => {
      state.items = state.items.filter((x) => x.timestamp !== action.payload);
    }
  }
});

const { addDownloadHistoryItem, deleteDownloadHistoryItem } =
  downloadHistorySlice.actions;

const selectDownloadHistory = (state: RootState) => state.downloadHistory.items;

export { addDownloadHistoryItem, deleteDownloadHistoryItem, selectDownloadHistory };

export default downloadHistorySlice.reducer;
