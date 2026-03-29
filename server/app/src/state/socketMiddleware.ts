import {
  AnyAction,
  Dispatch,
  Middleware,
  MiddlewareAPI,
  PayloadAction
} from "@reduxjs/toolkit";
import { addDownloadHistoryItem } from "./downloadHistorySlice";
import { deleteHistoryItem } from "./historySlice";
import {
  ConnectionResponse,
  DownloadResponse,
  MessageType,
  Notification,
  NotificationType,
  Response,
  SearchResponse
} from "./messages";
import { addNotification } from "./notificationSlice";
import {
  removePendingDownloadLabel,
  removeInFlightDownload,
  sendMessage,
  setConnectionState,
  setSearchResults,
  setUsername
} from "./stateSlice";
import { AppDispatch, RootState } from "./store";
import { displayNotification, downloadFile } from "./util";

// Web socket redux middleware.
// Listens to socket and dispatches handlers.
// Handles send_message actions by sending to socket.
export const websocketConn =
  (wsUrl: string): Middleware =>
  ({ dispatch, getState }: MiddlewareAPI<AppDispatch, RootState>) => {
    const socket = new WebSocket(wsUrl);

    socket.onopen = () => onOpen(dispatch);
    socket.onclose = () => onClose(dispatch);
    socket.onmessage = (message) => route(dispatch, getState, message);
    socket.onerror = (event) =>
      displayNotification({
        appearance: NotificationType.DANGER,
        title: "Unable to connect to server.",
        timestamp: new Date().getTime()
      });

    return (next: Dispatch<AnyAction>) => (action: PayloadAction<any>) => {
      // Send Message action? Send data to the socket.
      if (sendMessage.match(action)) {
        if (socket.readyState === socket.OPEN) {
          socket.send(action.payload.message);
        } else {
          displayNotification({
            appearance: NotificationType.WARNING,
            title: "Server connection closed. Reload page.",
            timestamp: new Date().getTime()
          });
        }
      }

      return next(action);
    };
  };

const onOpen = (dispatch: AppDispatch): void => {
  console.log("WebSocket connected.");
  dispatch(setConnectionState(false));
  dispatch(sendMessage({ type: MessageType.CONNECT, payload: {} }));
};

const onClose = (dispatch: AppDispatch): void => {
  console.log("WebSocket closed.");
  dispatch(setConnectionState(false));
};

const route = (
  dispatch: AppDispatch,
  getState: () => RootState,
  msg: MessageEvent<any>
): void => {
  const fileNameFromPath = (input?: string): string => {
    if (!input) return "";
    const normalized = input.replace(/\\/g, "/");
    const parts = normalized.split("/").filter((x) => x.length > 0);
    return parts.length === 0 ? input : parts[parts.length - 1];
  };

  const getNotif = (): Notification => {
    let response = JSON.parse(msg.data) as Response;
    const timestamp = new Date().getTime();
    const notification: Notification = {
      ...response,
      timestamp
    };

    switch (response.type) {
      case MessageType.STATUS:
        if (
          response.appearance === NotificationType.DANGER &&
          response.title.toLowerCase().includes("unable to join #ebooks")
        ) {
          dispatch(setConnectionState(false));
          dispatch(setUsername(""));
        }
        return notification;
      case MessageType.CONNECT:
        dispatch(setConnectionState(true));
        dispatch(setUsername((response as ConnectionResponse).name));
        return notification;
      case MessageType.SEARCH:
        dispatch(setSearchResults(response as SearchResponse));
        return notification;
      case MessageType.DOWNLOAD:
        const download = response as DownloadResponse;
        const state = getState();
        const pendingLabel = state.state.pendingDownloadLabels[0];
        const fileName = fileNameFromPath(
          download.downloadPath || download.detail
        );

        let displayName = fileName || download.detail || "Downloaded file";
        if (pendingLabel && (pendingLabel.title || pendingLabel.author)) {
          const title = (pendingLabel.title || "").trim();
          const authors = (pendingLabel.author || "").trim();

          const parts = [] as string[];
          if (title) {
            parts.push(title);
          }
          if (authors) {
            parts.push(authors);
          }
          if (fileName) {
            parts.push(fileName);
          }

          if (parts.length > 0) {
            displayName = parts.join(" - ");
          }
        }

        dispatch(
          addDownloadHistoryItem({
            name: displayName,
            downloadPath: download.downloadPath,
            timestamp
          })
        );
        downloadFile(download.downloadPath);
        dispatch(removePendingDownloadLabel());
        dispatch(removeInFlightDownload());
        return notification;
      case MessageType.RATELIMIT:
        dispatch(deleteHistoryItem());
        return notification;
      default:
        console.error(response);
        return {
          appearance: NotificationType.DANGER,
          title: "Unknown message type. See console.",
          timestamp
        };
    }
  };

  const notif = getNotif();
  dispatch(addNotification(notif));
  displayNotification(notif);
};
