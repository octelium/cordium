import * as UserPB from "@octelium/apis/main/userv1";
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
