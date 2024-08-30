function showForm() {
  var html = HtmlService.createHtmlOutputFromFile('contributor-form')
      .setTitle('Sidebar Form')
      .setWidth(300);
  SpreadsheetApp.getUi().showSidebar(html);
}

function getCellData() {
  var sheet = SpreadsheetApp.getActiveSpreadsheet();
  var cell = sheet.getActiveCell();
  return cell.getValue();
}

function populateCell(data) {
  var sheet = SpreadsheetApp.getActiveSpreadsheet();
  var cell = sheet.getActiveCell();
  cell.setValue(data);
}

