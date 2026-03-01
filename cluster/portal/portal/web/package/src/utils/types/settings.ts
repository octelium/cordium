import * as UserPB from "@/apis/userv1/userv1";
interface Settings {
  wideTerminal?: boolean;
  terminalFontSize?: number;
  itemsPerPage?: number;
  status?: UserPB.GetStatusResponse;
  personalSpaceUID?: string;
  autoCreateFirstTerminal?: boolean;
  // itemsPerPageNavigator?: number;
}

export default Settings;
