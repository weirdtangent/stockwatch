#!/usr/bin/perl

use DBI;
use strict;

die "usage: $0 <exchange>"
  unless scalar(@ARGV) == 1;

my $exchange = $ARGV[0];
my $import = "tickers/$exchange.txt";

my $dbh = DBI->connect(
  "dbi:CSV:", undef, undef, {
    f_ext      => ".txt/r",
    f_dir      => "data"
    RaiseError => 1,
  }
) or die "Cannot connect: $DBI::errstr";

#
# "exchange_id","exchange_short_name","exchange_name"
# "1","AMEX","American Stock Exchange"
# "2","ASX","Australian Securities Exchange"
# "3","CBOT","Chicago Board of Trade"
# "4","CFE","Chicago Futures Exchange"
# "5","CME","Chicago Merchantile Exchange"
# "6","COMEX","New York Commodity Exchange"
#
#
my $exchange_id;
my $sth = $dbh->prepare("select exchange_id from exchange where exchange_short_name='$exchange'");
$sth->execute;
$sth->bind_columns (\my ($id));
while ($sth->fetch) {
  $exchange_id = $id;
}
$sth->finish;
$dbh->disconnect;

die "Exchange $exchange not found in table" unless $exchange_id;




$dbh = DBI->connect(
  "dbi:CSV:", undef, undef, {
    f_ext      => ".txt/r",
    f_dir      => "data"
    sep_char   => "\t",
    RaiseError => 1,
  }
) or die "Cannot connect: $DBI::errstr";
#
# Symbol  Description
# AAA     First Priority Clo Bond ETF
# AAAU    Goldman Sachs Physical Gold ETF
# AADR    Advisorshares Dorsey Wright ETF
# AAMC    Altisource Asset
# AAU     Almaden Minerals
# ABEQ    Absolute Core Strategy ETF
# ACES    Alps Clean Energy ETF
#
$sth = $dbh->prepare("select * from $import");
$sth->execute;
$sth->bind_columns (\my ($symbol, $name));
while ($sth->fetch) {
  next if $symbol eq 'Symbol';
  printf "INSERT INTO ticker SET ticker_symbol='%s', exchange_id=%d, ticker_name='%s';\n",
    $symbol, $exchange_id, $name;
}
$sth->finish;
 
$dbh->disconnect;

