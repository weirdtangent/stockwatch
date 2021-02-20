#!/usr/bin/perl

use DBI;
use strict;

my $metastock = $ARGV[0] || die "usage: $0 <metastock_file.csv>";
my $stock;

my $dbh = DBI->connect(
  "dbi:CSV:", undef, undef, {
    f_ext      => ".txt/r",
    f_dir      => "data"
    RaiseError => 1,
  }
) or die "Cannot connect: $DBI::errstr";

#
# "ticker_id","ticker_symbol","exchange_id","name","country_id"
# "1","AAMC","1","Altisource Asset Management Corp Com","NULL"
# "2","AAU","1","Almaden Minerals Ltd. Common Shares","NULL"
# "3","ACU","1","Acme United Corporation. Common Stock","NULL"
# "4","ACY","1","AeroCentury Corp. Common Stock","NULL"
#
my $sth = $dbh->prepare("select * from ticker where exchange_id IN (1,15,18)");
$sth->execute;
$sth->bind_columns (\my ($ticker_id, $ticker_symbol, $exchange_id, $name, $country_id));
while ($sth->fetch) {
  $stock->{$ticker_symbol} = $ticker_id;
}
$sth->finish;

#
# A,20210101,118.49,118.49,118.49,118.49,0
# AA,20210101,23.05,23.05,23.05,23.05,0
# AAA,20210101,25.07,25.07,25.07,25.07,0
# AAAU,20210101,18.94,18.94,18.94,18.94,0
#
$sth = $dbh->prepare("select * from $metastock");
$sth->execute;
$sth->bind_columns (\my ($ticker_symbol, $date, $open, $high, $low, $close, $volume));
while ($sth->fetch) {
  if (exists $stock->{$ticker_symbol}) {
    my ($yyyy,$mm,$dd) = $date =~ /^(\d\d\d\d)(\d\d)(\d\d)$/;
    printf "INSERT INTO daily SET ticker_id=%d, price_date='%s', open_price=%f, high_price=%f, low_price=%f, close_price=%f, volume=%d;\n",
      $stock->{$ticker_symbol}, "$yyyy-$mm-$dd", $open, $high, $low, $close, $volume;
  }
}
$sth->finish;
 
$dbh->disconnect;

