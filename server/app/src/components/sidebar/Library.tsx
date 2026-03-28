import {
  Badge,
  Button,
  Center,
  Menu,
  Stack,
  Text,
  Tooltip
} from "@mantine/core";
import { AnimatePresence, motion } from "framer-motion";
import { Book as BookIcon, Download, Trash } from "phosphor-react";
import {
  deleteDownloadHistoryItem,
  DownloadHistoryItem,
  selectDownloadHistory
} from "../../state/downloadHistorySlice";
import { useAppDispatch, useAppSelector } from "../../state/store";
import { downloadFile } from "../../state/util";
import { defaultAnimation } from "../../utils/animation";
import { useSidebarButtonStyle } from "./styles";

export default function Library() {
  const data = useAppSelector(selectDownloadHistory);

  if (data.length === 0) {
    return (
      <Center>
        <Text color="dimmed" size="sm">
          No previous downloads.
        </Text>
      </Center>
    );
  }

  return (
    <Stack spacing="xs">
      <AnimatePresence mode="popLayout">
        {data?.map((book) => (
          <motion.div {...defaultAnimation} key={book.timestamp.toString()}>
            <LibraryCard key={book.timestamp.toString()} book={book} />
          </motion.div>
        ))}
      </AnimatePresence>
    </Stack>
  );
}

interface LibraryCardProps {
  book: DownloadHistoryItem;
}

function LibraryCard({ book }: LibraryCardProps) {
  const { classes } = useSidebarButtonStyle({});
  const dispatch = useAppDispatch();

  return (
    <Menu shadow="md">
      <Menu.Target>
        <Tooltip label={book.name} openDelay={1_000}>
          <Button
            key={book.name}
            classNames={classes}
            radius="sm"
            variant="outline"
            fullWidth
            leftIcon={<BookIcon weight="bold" size={18} />}
            rightIcon={
              <Badge color="brand" radius="sm" size="sm" variant="light">
                {new Date(book.timestamp).toLocaleDateString("en-US")}
              </Badge>
            }>
            {book.name}
          </Button>
        </Tooltip>
      </Menu.Target>

      <Menu.Dropdown>
        <Menu.Item
          icon={<Download weight="bold" />}
          disabled={!book.downloadPath}
          onClick={() => downloadFile(book.downloadPath)}>
          Download
        </Menu.Item>

        <Menu.Item
          color="red"
          icon={<Trash size={18} weight="bold" />}
          onClick={() => dispatch(deleteDownloadHistoryItem(book.timestamp))}>
          Remove from history
        </Menu.Item>
      </Menu.Dropdown>
    </Menu>
  );
}
